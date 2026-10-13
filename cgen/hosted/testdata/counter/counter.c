/* A hand-written stand-in for main.eye compiled, until cgen writes the
 * hosting interface (eye_host.h) itself. It runs main, the tests,
 * Counter.add, Total.add and the job tick, calls back into the runner as
 * compiled code will (entity calls and reads, starts, publishing, host
 * functions), and reads and writes values in the interpreter's encoding
 * (interp/save.go), which is all the runner sees of them.
 *
 * Values are a small tree of what the encoding holds: strings, integers,
 * (), booleans, records, variants, lists and runs. A job's frame is a
 * record { at: Int, id: String }: where the job is, and its variable. */
#include <stdarg.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "eye_host.h"

/* ---- types, by index, as the tables name them */

enum {
	T_STRING, T_INT, T_UNIT, T_BOOL, T_COUNTER, T_COUNTER_SNAPSHOT, T_TOTAL, T_TOTAL_SNAPSHOT,
	T_FRAME, T_OPTION_STRING, T_LIST_STRING, T_RUN, T_COUNTED, NTYPES
};
enum { E_COUNTER, E_TOTAL };
enum { H_ENV_GET, H_PRINT, H_EXPECT };

/* ---- values */

enum { V_STRING, V_INT, V_UNIT, V_BOOL, V_RECORD, V_VARIANT, V_LIST, V_RUN };

typedef struct val {
	int kind;
	long refs;
	int64_t i;     /* V_INT, V_BOOL, V_RUN */
	char *s;       /* V_STRING; V_RECORD: its type's name, or NULL; V_VARIANT: its name */
	size_t n;      /* V_STRING: its length */
	int count;     /* V_RECORD, V_VARIANT, V_LIST: how many parts */
	char **names;  /* V_RECORD: the fields' names */
	struct val **parts;
	eye_val args[1]; /* a frame's step arguments, owned by the frame */
} val;

static eye_val H(val *v) { return (eye_val)(uintptr_t)v; }
static val *V(eye_val h) { return (val *)(uintptr_t)h; }

static char fault_msg[256];

static eye_status fault(const char *msg) {
	snprintf(fault_msg, sizeof fault_msg, "%s", msg);
	return EYE_FAULT;
}

const char *eye_fault(eye_rt *rt) {
	(void)rt;
	return fault_msg;
}

static val *mk(int kind) {
	val *v = calloc(1, sizeof(val));
	v->kind = kind;
	v->refs = 1;
	return v;
}

static val *mkint(int64_t i) {
	val *v = mk(V_INT);
	v->i = i;
	return v;
}

static val *mkbool(int b) {
	val *v = mk(V_BOOL);
	v->i = b != 0;
	return v;
}

static val *mkstr(const char *s) {
	val *v = mk(V_STRING);
	v->n = strlen(s);
	v->s = malloc(v->n + 1);
	memcpy(v->s, s, v->n + 1);
	return v;
}

void eye_retain(eye_val h) {
	if (h) V(h)->refs++;
}

void eye_release(eye_val h) {
	val *v = V(h);
	if (!v || --v->refs > 0) return;
	for (int i = 0; i < v->count; i++) {
		if (v->names) free(v->names[i]);
		eye_release(H(v->parts[i]));
	}
	free(v->names);
	free(v->parts);
	free(v->s);
	if (v->args[0]) eye_release(v->args[0]);
	free(v);
}

/* A record of n fields, given as name, value pairs; it takes the values. */
static val *mkrecord(const char *type, int n, ...) {
	val *v = mk(V_RECORD);
	v->s = type ? strdup(type) : NULL;
	v->count = n;
	v->names = calloc((size_t)n, sizeof(char *));
	v->parts = calloc((size_t)n, sizeof(val *));
	va_list ap;
	va_start(ap, n);
	for (int i = 0; i < n; i++) {
		v->names[i] = strdup(va_arg(ap, const char *));
		v->parts[i] = va_arg(ap, val *);
	}
	va_end(ap);
	return v;
}

/* A record's field, or NULL. */
static val *field(val *r, const char *name) {
	if (!r || r->kind != V_RECORD) return NULL;
	for (int i = 0; i < r->count; i++)
		if (strcmp(r->names[i], name) == 0) return r->parts[i];
	return NULL;
}

static val *retained(val *v) {
	v->refs++;
	return v;
}

/* ---- the encoding: a tag byte, then varints and strings */

typedef struct {
	uint8_t *p;
	size_t n, cap;
} wbuf;

static void put(wbuf *b, uint8_t c) {
	if (b->n == b->cap) {
		b->cap = b->cap ? b->cap * 2 : 64;
		b->p = realloc(b->p, b->cap);
	}
	b->p[b->n++] = c;
}

static void put_uvarint(wbuf *b, uint64_t x) {
	while (x >= 0x80) {
		put(b, (uint8_t)x | 0x80);
		x >>= 7;
	}
	put(b, (uint8_t)x);
}

static void put_varint(wbuf *b, int64_t x) { put_uvarint(b, ((uint64_t)x << 1) ^ (uint64_t)(x >> 63)); }

static void put_string(wbuf *b, const char *s, size_t n) {
	put_uvarint(b, n);
	for (size_t i = 0; i < n; i++) put(b, (uint8_t)s[i]);
}

static void put_value(wbuf *b, val *v) {
	switch (v->kind) {
	case V_STRING:
		put(b, 's');
		put_string(b, v->s, v->n);
		break;
	case V_INT:
		put(b, 'i');
		put_varint(b, v->i);
		break;
	case V_UNIT:
		put(b, 'u');
		break;
	case V_BOOL:
		put(b, v->i ? 'T' : 'F');
		break;
	case V_RUN:
		put(b, 'r');
		put_varint(b, v->i);
		break;
	case V_RECORD:
		if (v->s) {
			put(b, 'N');
			put_string(b, v->s, strlen(v->s));
		} else {
			put(b, 'R');
		}
		put_uvarint(b, (uint64_t)v->count);
		for (int i = 0; i < v->count; i++) {
			put_string(b, v->names[i], strlen(v->names[i]));
			put_value(b, v->parts[i]);
		}
		break;
	case V_VARIANT:
	case V_LIST:
		if (v->kind == V_VARIANT) {
			put(b, 'V');
			put_string(b, v->s, strlen(v->s));
		} else {
			put(b, 'L');
		}
		put_uvarint(b, (uint64_t)v->count);
		for (int i = 0; i < v->count; i++) put_value(b, v->parts[i]);
		break;
	}
}

eye_status eye_encode(eye_rt *rt, int32_t type, eye_val h, eye_buf *out) {
	(void)rt;
	if (type < 0 || type >= NTYPES || !h) return fault("counter.c can't encode that");
	wbuf b = {0};
	put_value(&b, V(h));
	out->p = b.p;
	out->n = b.n;
	return EYE_OK;
}

typedef struct {
	const uint8_t *p;
	size_t n, at;
	int bad;
} rbuf;

static uint64_t get_uvarint(rbuf *r) {
	uint64_t x = 0;
	for (int shift = 0; shift < 64; shift += 7) {
		if (r->at >= r->n) break;
		uint8_t c = r->p[r->at++];
		x |= (uint64_t)(c & 0x7f) << shift;
		if (c < 0x80) return x;
	}
	r->bad = 1;
	return 0;
}

static int64_t get_varint(rbuf *r) {
	uint64_t u = get_uvarint(r);
	return (int64_t)(u >> 1) ^ -(int64_t)(u & 1);
}

static char *get_text(rbuf *r, size_t *len) {
	uint64_t n = get_uvarint(r);
	if (r->bad || n > r->n - r->at) {
		r->bad = 1;
		return NULL;
	}
	char *s = malloc(n + 1);
	memcpy(s, r->p + r->at, n);
	s[n] = 0;
	r->at += n;
	if (len) *len = n;
	return s;
}

static val *get_value(rbuf *r, int depth);

static int get_parts(rbuf *r, val *v, int depth, int named) {
	uint64_t n = get_uvarint(r);
	if (r->bad || n > r->n) return 0;
	v->count = (int)n;
	v->parts = calloc(n ? n : 1, sizeof(val *));
	if (named) v->names = calloc(n ? n : 1, sizeof(char *));
	for (uint64_t i = 0; i < n; i++) {
		if (named && !(v->names[i] = get_text(r, NULL))) return 0;
		if (!(v->parts[i] = get_value(r, depth + 1))) return 0;
	}
	return 1;
}

static val *get_value(rbuf *r, int depth) {
	if (r->at >= r->n || depth > 64) {
		r->bad = 1;
		return NULL;
	}
	uint8_t tag = r->p[r->at++];
	val *v = NULL;
	switch (tag) {
	case 's':
		v = mk(V_STRING);
		v->s = get_text(r, &v->n);
		break;
	case 'i':
		v = mkint(get_varint(r));
		break;
	case 'u':
		v = mk(V_UNIT);
		break;
	case 'T':
	case 'F':
		v = mkbool(tag == 'T');
		break;
	case 'r':
		v = mk(V_RUN);
		v->i = get_varint(r);
		break;
	case 'N':
	case 'R':
		v = mk(V_RECORD);
		if (tag == 'N') v->s = get_text(r, NULL);
		if (!get_parts(r, v, depth, 1)) r->bad = 1;
		break;
	case 'V':
		v = mk(V_VARIANT);
		v->s = get_text(r, NULL);
		if (!get_parts(r, v, depth, 0)) r->bad = 1;
		break;
	case 'L':
		v = mk(V_LIST);
		if (!get_parts(r, v, depth, 0)) r->bad = 1;
		break;
	default:
		r->bad = 1;
	}
	if (r->bad && v) {
		eye_release(H(v));
		return NULL;
	}
	return v;
}

eye_status eye_decode(eye_rt *rt, int32_t type, eye_bytes in, eye_val *out) {
	(void)rt;
	if (type < 0 || type >= NTYPES) return fault("counter.c has no such type");
	rbuf r = {in.p, in.n, 0, 0};
	val *v = get_value(&r, 0);
	if (!v || r.bad || r.at != r.n) {
		if (v) eye_release(H(v));
		return fault("counter.c can't read this value");
	}
	*out = H(v);
	return EYE_OK;
}

/* ---- calls back into the runner */

static eye_status host1(eye_rt *rt, int32_t fn, val *arg) {
	eye_val args[1] = {H(arg)};
	eye_val result = EYE_NONE;
	eye_status s = eye_go_host(rt, fn, args, 1, &result);
	eye_release(H(arg));
	if (result) eye_release(result);
	return s;
}

/* print's values are its arguments: one here, a String. */
static eye_status print(eye_rt *rt, const char *text) { return host1(rt, H_PRINT, mkstr(text)); }

static eye_status expect(eye_rt *rt, int cond) { return host1(rt, H_EXPECT, mkbool(cond)); }

/* Calls entity e's only method, add(n), on id, and gives what it returns. */
static eye_status call_add(eye_rt *rt, int32_t e, const char *id, int64_t n, int64_t *out) {
	val *idv = mkstr(id);
	eye_val args[1] = {H(mkint(n))};
	eye_val result = EYE_NONE;
	eye_status s = eye_go_entity_call(rt, e, H(idv), 0, args, 1, &result);
	eye_release(H(idv));
	eye_release(args[0]);
	if (s == EYE_OK) {
		*out = V(result)->i;
		eye_release(result);
	}
	return s;
}

/* Reads an Int field of entity e's snapshot. */
static eye_status read_field(eye_rt *rt, int32_t e, const char *id, const char *name, int64_t *out) {
	val *idv = mkstr(id);
	eye_val snap = EYE_NONE;
	eye_status s = eye_go_entity_read(rt, e, H(idv), &snap);
	eye_release(H(idv));
	if (s != EYE_OK) return s;
	val *f = field(V(snap), name);
	if (!f || f->kind != V_INT) {
		eye_release(snap);
		return fault("the snapshot has no such Int field");
	}
	*out = f->i;
	eye_release(snap);
	return EYE_OK;
}

/* ---- main and the tests */

eye_status eye_run_main(eye_rt *rt, eye_val args, eye_val *result) {
	(void)args;
	int64_t count;
	eye_status s = read_field(rt, E_COUNTER, "a", "count", &count);
	if (s != EYE_OK) return s;
	char line[64];
	snprintf(line, sizeof line, "count %lld", (long long)count);
	if ((s = print(rt, line)) != EYE_OK) return s;
	val *id = mkstr("a"), *key = mkstr("a");
	eye_val startArgs[1] = {H(id)};
	eye_val run = EYE_NONE;
	s = eye_go_start(rt, 0, startArgs, 1, H(key), &run);
	eye_release(H(id));
	eye_release(H(key));
	if (s != EYE_OK) return s;
	eye_release(run);
	if ((s = print(rt, "started")) != EYE_OK) return s;
	*result = H(mk(V_UNIT));
	return EYE_OK;
}

eye_status eye_run_test(eye_rt *rt, int32_t test) {
	int64_t n;
	eye_status s;
	switch (test) {
	case 0: /* counts and totals */
		if ((s = call_add(rt, E_COUNTER, "t", 2, &n)) != EYE_OK) return s;
		if ((s = expect(rt, n == 2)) != EYE_OK) return s;
		if ((s = read_field(rt, E_TOTAL, "all", "sum", &n)) != EYE_OK) return s;
		return expect(rt, n == 2);
	case 1: /* a failing expect stops the test */
		if ((s = call_add(rt, E_COUNTER, "f", 1, &n)) != EYE_OK) return s;
		return expect(rt, n == 5);
	}
	return fault("counter.c has no such test");
}

/* ---- the entities' methods */

/* A state of one Int field after the identity, changed in place when the
 * runner held the only reference. */
static val *own(eye_val *state) {
	val *s = V(*state);
	if (s->refs == 1) return s;
	val *c = mkrecord(NULL, 2, s->names[0], retained(s->parts[0]), s->names[1], mkint(s->parts[1]->i));
	eye_release(*state);
	*state = H(c);
	return c;
}

eye_status eye_call_method(eye_rt *rt, int32_t entity, int32_t method, eye_val *state,
                           const eye_val *args, int32_t nargs, eye_val *result) {
	if (method != 0 || nargs != 1) return fault("counter.c has only add(n)");
	val *s = V(*state);
	if (s->kind != V_RECORD || s->count != 2 || s->parts[1]->kind != V_INT) return fault("not a state counter.c knows");
	int64_t n = V(args[0])->i;
	switch (entity) {
	case E_COUNTER: {
		s = own(state);
		s->parts[1]->i += n;
		/* bus.counted.publish({ id, count }) */
		val *ev = mkrecord("bus.Counted", 2, "id", retained(s->parts[0]), "count", mkint(s->parts[1]->i));
		eye_status st = eye_go_publish(rt, 0, H(ev));
		eye_release(H(ev));
		if (st != EYE_OK) return st;
		/* Total("all").add(n), in this transaction */
		int64_t sum;
		if ((st = call_add(rt, E_TOTAL, "all", n, &sum)) != EYE_OK) return st;
		*result = H(mkint(s->parts[1]->i));
		return EYE_OK;
	}
	case E_TOTAL:
		s = own(state);
		s->parts[1]->i += n;
		*result = H(mkint(s->parts[1]->i));
		return EYE_OK;
	}
	return fault("counter.c has no such entity");
}

/* ---- tick: sleep!(2 s), env.get!("TICK"), then Counter(id).add!(1) */

eye_status eye_job_frame(eye_rt *rt, int32_t job, const eye_val *args, int32_t nargs, eye_val *frame) {
	(void)rt;
	if (job != 0 || nargs != 1) return fault("counter.c has only tick(id)");
	*frame = H(mkrecord(NULL, 2, "at", mkint(0), "id", retained(V(args[0]))));
	return EYE_OK;
}

eye_status eye_step_job(eye_rt *rt, int32_t job, eye_val *frame, eye_val resumed,
                        eye_step *step, eye_val *result) {
	(void)rt;
	(void)resumed;
	if (job != 0) return fault("counter.c has only tick");
	val *f = V(*frame);
	val *at = field(f, "at"), *id = field(f, "id");
	if (!at || !id || f->refs != 1) return fault("not a frame of tick's");
	memset(step, 0, sizeof *step);
	if (f->args[0]) {
		eye_release(f->args[0]);
		f->args[0] = EYE_NONE;
	}
	switch (at->i++) {
	case 0:
		step->kind = EYE_STEP_SLEEP;
		step->sleep_ns = 2000000000LL;
		step->result_type = T_UNIT;
		return EYE_WAITING;
	case 1:
		f->args[0] = H(mkstr("TICK"));
		step->kind = EYE_STEP_HOST;
		step->fn = H_ENV_GET;
		step->nargs = 1;
		step->args = f->args;
		step->result_type = T_OPTION_STRING;
		return EYE_WAITING;
	case 2:
		f->args[0] = H(mkint(1));
		step->kind = EYE_STEP_CALL;
		step->entity = E_COUNTER;
		step->method = 0;
		step->id = H(id);
		step->nargs = 1;
		step->args = f->args;
		step->result_type = T_INT;
		return EYE_WAITING;
	case 3:
		*result = H(mk(V_UNIT));
		return EYE_OK;
	}
	return fault("tick's frame is at a place it doesn't have");
}

/* ---- the tables */

static const int32_t one_int[] = {T_INT};
static const int32_t one_string[] = {T_STRING};
static const int32_t one_bool[] = {T_BOOL};
static const eye_method_info counter_methods[] = {{"add", 1, one_int, T_INT}};
static const eye_method_info total_methods[] = {{"add", 1, one_int, T_INT}};
static const eye_entity_info entities[] = {
	{"Counter", T_STRING, T_COUNTER, T_COUNTER_SNAPSHOT, 1, counter_methods},
	{"Total", T_STRING, T_TOTAL, T_TOTAL_SNAPSHOT, 1, total_methods},
};
static const eye_job_info jobs[] = {{"tick", 1, one_string, T_FRAME, T_UNIT}};
static const eye_host_info hosts[] = {
	{"env.get", 1, one_string, T_OPTION_STRING},
	{"print", 1, one_string, T_UNIT},
	{"expect", 1, one_bool, T_UNIT},
};
static const eye_event_info events[] = {{"bus.counted", T_COUNTED}};
static const char *const tests[] = {"counts and totals", "a failing expect stops the test"};
static const char *const types[] = {
	"String", "Int", "()", "Bool", "Counter", "Counter's snapshot", "Total", "Total's snapshot",
	"tick's frame", "Option(String)", "List(String)", "Run", "bus.Counted",
};

static const eye_program_info program = {
	.version = EYE_HOST_VERSION,
	.ntypes = NTYPES,
	.types = types,
	.nentities = 2,
	.entities = entities,
	.njobs = 1,
	.jobs = jobs,
	.ntests = 2,
	.tests = tests,
	.nhosts = 3,
	.hosts = hosts,
	.nevents = 1,
	.events = events,
	.has_main = 1,
	.main_args_type = T_LIST_STRING,
	.main_result_type = T_UNIT,
	.key_type = T_STRING,
	.run_type = T_RUN,
};

const eye_program_info *eye_program(void) { return &program; }
