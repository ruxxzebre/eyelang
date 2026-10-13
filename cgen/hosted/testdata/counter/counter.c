/* A hand-written stand-in for main.eye compiled, until cgen writes the
 * hosting interface (eye_host.h) itself. It runs Counter.add and the job
 * tick, and reads and writes values in the interpreter's encoding
 * (interp/save.go), which is all the runner sees of them.
 *
 * Types, by index: 0 String, 1 Int, 2 (), 3 Counter's state, 4 tick's
 * frame. A frame is saved as a record { at: Int, id: String }: where the
 * job is, and its variable. */
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "eye_host.h"

enum { T_STRING, T_INT, T_UNIT, T_STATE, T_FRAME };
enum { V_STRING, V_INT, V_UNIT, V_STATE, V_FRAME };

typedef struct val {
	int kind;
	long refs;
	int64_t i;      /* V_INT; V_FRAME: where the job is */
	char *s;        /* V_STRING */
	size_t n;       /* V_STRING */
	struct val *id; /* V_STATE, V_FRAME */
	int64_t count;  /* V_STATE */
	eye_val args[1]; /* V_FRAME: the step's arguments, owned here */
} val;

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

void eye_retain(eye_val h) {
	if (h) ((val *)(uintptr_t)h)->refs++;
}

void eye_release(eye_val h) {
	val *v = (val *)(uintptr_t)h;
	if (!v || --v->refs > 0) return;
	free(v->s);
	if (v->id) eye_release((eye_val)(uintptr_t)v->id);
	if (v->args[0]) eye_release(v->args[0]);
	free(v);
}

static eye_val H(val *v) { return (eye_val)(uintptr_t)v; }
static val *V(eye_val h) { return (val *)(uintptr_t)h; }

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

static void put_varint(wbuf *b, int64_t x) {
	put_uvarint(b, ((uint64_t)x << 1) ^ (uint64_t)(x >> 63));
}

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
	case V_STATE:
		put(b, 'R');
		put_uvarint(b, 2);
		put_string(b, "id", 2);
		put_value(b, v->id);
		put_string(b, "count", 5);
		put(b, 'i');
		put_varint(b, v->count);
		break;
	case V_FRAME:
		put(b, 'R');
		put_uvarint(b, 2);
		put_string(b, "at", 2);
		put(b, 'i');
		put_varint(b, v->i);
		put_string(b, "id", 2);
		put_value(b, v->id);
		break;
	}
}

eye_status eye_encode(eye_rt *rt, int32_t type, eye_val h, eye_buf *out) {
	(void)rt;
	(void)type;
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
		if (r->at >= r->n) {
			r->bad = 1;
			return 0;
		}
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

static int get_tag(rbuf *r, uint8_t want) {
	if (r->at >= r->n || r->p[r->at] != want) {
		r->bad = 1;
		return 0;
	}
	r->at++;
	return 1;
}

static val *get_string(rbuf *r) {
	if (!get_tag(r, 's')) return NULL;
	uint64_t n = get_uvarint(r);
	if (r->bad || n > r->n - r->at) {
		r->bad = 1;
		return NULL;
	}
	val *v = mk(V_STRING);
	v->s = malloc(n + 1);
	memcpy(v->s, r->p + r->at, n);
	v->s[n] = 0;
	v->n = n;
	r->at += n;
	return v;
}

static int get_name(rbuf *r, const char *want) {
	uint64_t n = get_uvarint(r);
	size_t len = strlen(want);
	if (r->bad || n != len || n > r->n - r->at || memcmp(r->p + r->at, want, len) != 0) {
		r->bad = 1;
		return 0;
	}
	r->at += n;
	return 1;
}

static int64_t get_int(rbuf *r) {
	if (!get_tag(r, 'i')) return 0;
	return get_varint(r);
}

eye_status eye_decode(eye_rt *rt, int32_t type, eye_bytes in, eye_val *out) {
	(void)rt;
	rbuf r = {in.p, in.n, 0, 0};
	val *v = NULL;
	switch (type) {
	case T_STRING:
		v = get_string(&r);
		break;
	case T_INT:
		v = mkint(get_int(&r));
		break;
	case T_UNIT:
		get_tag(&r, 'u');
		v = mk(V_UNIT);
		break;
	case T_STATE:
		/* The store keeps a state as a record of its fields, in the
		 * order they are declared: the identity, then count. */
		v = mk(V_STATE);
		if (get_tag(&r, 'R') && get_uvarint(&r) == 2 && get_name(&r, "id")) {
			v->id = get_string(&r);
			if (get_name(&r, "count")) v->count = get_int(&r);
		} else {
			r.bad = 1;
		}
		break;
	case T_FRAME:
		v = mk(V_FRAME);
		if (get_tag(&r, 'R') && get_uvarint(&r) == 2 && get_name(&r, "at")) {
			v->i = get_int(&r);
			if (get_name(&r, "id")) v->id = get_string(&r);
		} else {
			r.bad = 1;
		}
		break;
	default:
		return fault("counter.c has no such type");
	}
	if (r.bad || r.at != r.n) {
		if (v) eye_release(H(v));
		return fault("counter.c can't read this value as the type asked for");
	}
	*out = H(v);
	return EYE_OK;
}

/* ---- Counter.add */

eye_status eye_call_method(eye_rt *rt, int32_t entity, int32_t method, eye_val *state,
                           const eye_val *args, int32_t nargs, eye_val *result) {
	(void)rt;
	if (entity != 0 || method != 0 || nargs != 1) return fault("counter.c has only Counter.add(n)");
	val *s = V(*state);
	if (s->refs > 1) {
		/* Shared: the change goes into a copy. */
		val *c = mk(V_STATE);
		c->id = s->id;
		eye_retain(H(c->id));
		c->count = s->count;
		eye_release(*state);
		s = c;
	}
	s->count += V(args[0])->i;
	*state = H(s);
	*result = H(mkint(s->count));
	return EYE_OK;
}

/* ---- tick: sleep!(2 s), then Counter(id).add!(1) */

eye_status eye_job_frame(eye_rt *rt, int32_t job, const eye_val *args, int32_t nargs, eye_val *frame) {
	(void)rt;
	if (job != 0 || nargs != 1) return fault("counter.c has only tick(id)");
	val *f = mk(V_FRAME);
	f->id = V(args[0]);
	eye_retain(args[0]);
	*frame = H(f);
	return EYE_OK;
}

eye_status eye_step_job(eye_rt *rt, int32_t job, eye_val *frame, eye_val resumed,
                        eye_step *step, eye_val *result) {
	(void)rt;
	(void)resumed;
	if (job != 0) return fault("counter.c has only tick");
	val *f = V(*frame);
	if (f->refs > 1) {
		val *c = mk(V_FRAME);
		c->i = f->i;
		c->id = f->id;
		eye_retain(H(c->id));
		eye_release(*frame);
		f = c;
		*frame = H(f);
	}
	memset(step, 0, sizeof *step);
	switch (f->i) {
	case 0:
		f->i = 1;
		step->kind = EYE_STEP_SLEEP;
		step->sleep_ns = 2000000000LL;
		step->result_type = T_UNIT;
		return EYE_WAITING;
	case 1:
		f->i = 2;
		if (f->args[0]) eye_release(f->args[0]);
		f->args[0] = H(mkint(1));
		step->kind = EYE_STEP_CALL;
		step->entity = 0;
		step->method = 0;
		step->id = H(f->id);
		step->nargs = 1;
		step->args = f->args;
		step->result_type = T_INT;
		return EYE_WAITING;
	case 2:
		*result = H(mk(V_UNIT));
		return EYE_OK;
	}
	return fault("tick's frame is at a place it doesn't have");
}

/* ---- the tables */

static const int32_t add_params[] = {T_INT};
static const eye_method_info counter_methods[] = {{"add", 1, add_params, T_INT}};
static const eye_entity_info entities[] = {{"Counter", T_STRING, T_STATE, 1, counter_methods}};
static const int32_t tick_params[] = {T_STRING};
static const eye_job_info jobs[] = {{"tick", 1, tick_params, T_FRAME, T_UNIT}};
static const char *const types[] = {"String", "Int", "()", "Counter", "tick's frame"};

static const eye_program_info program = {
	.version = EYE_HOST_VERSION,
	.ntypes = 5,
	.types = types,
	.nentities = 1,
	.entities = entities,
	.njobs = 1,
	.jobs = jobs,
};

const eye_program_info *eye_program(void) { return &program; }
