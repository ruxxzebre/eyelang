/* The compiled side of a program the Go runner hosts (ROADMAP 7.6;
 * compilation.md, "Compiled jobs"): handles, faults, callbacks and frames.
 * Generated code writes the rest of eye_host.h's entry points, one for each
 * of the program's methods and jobs. Compiled only under -DEY_HOSTED; at -O2
 * the runtime is one translation unit, so its static names start with eyh_. */
#ifdef EY_HOSTED

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "eye.h"
#include "hosted.h"

/* ---------------------------------------------------------------- handles */

typedef struct eyh_val {
	int refs;
	eye_val *args; /* a frame's step arguments, which it owns */
	int nargs;
	size_t n;
	uint8_t b[];
} eyh_val;

#define EYH(h) ((eyh_val *)(uintptr_t)(h))

static eye_val eyh_bytes(const void *p, size_t n) {
	eyh_val *v = malloc(sizeof(eyh_val) + n + 1);
	if (!v) abort();
	v->refs = 1;
	v->args = NULL;
	v->nargs = 0;
	v->n = n;
	if (n > 0) memcpy(v->b, p, n);
	return (eye_val)(uintptr_t)v;
}

void eye_retain(eye_val h) {
	if (h != EYE_NONE) EYH(h)->refs++;
}

void eye_release(eye_val h) {
	if (h == EYE_NONE || --EYH(h)->refs > 0) return;
	for (int i = 0; i < EYH(h)->nargs; i++) eye_release(EYH(h)->args[i]);
	free(EYH(h)->args);
	free(EYH(h));
}

eye_status eye_encode(eye_rt *rt, int32_t type, eye_val v, eye_buf *out) {
	(void)rt;
	(void)type;
	out->n = EYH(v)->n;
	out->p = malloc(out->n + 1);
	if (!out->p) abort();
	memcpy(out->p, EYH(v)->b, out->n);
	return EYE_OK;
}

eye_status eye_decode(eye_rt *rt, int32_t type, eye_bytes in, eye_val *out) {
	(void)rt;
	(void)type;
	*out = eyh_bytes(in.p, in.n);
	return EYE_OK;
}

eye_val eyh_enc(ey_encoder e, const void *v) {
	ey_buf b = {0};
	e(&b, v);
	eye_val h = eyh_bytes(b.b, (size_t)b.len);
	free(b.b);
	return h;
}

void eyh_dec(eye_val h, ey_decoder d, void *out) {
	ey_rd r = {EYH(h)->b, EYH(h)->b + EYH(h)->n, NULL};
	if (!d(&r, out)) ey_fault("<runner>", r.err ? r.err : "corrupt saved value");
}

/* ---------------------------------------------------------------- faults */

static _Thread_local eyh_trap *eyh_top;
static _Thread_local char *eyh_message;
_Thread_local eyj *eyj_cur;

/* How much of the stack a call from the runner may use. The stack is the
 * thread's Go gave the call: a Linux thread's is set in eyh_stack below. */
#define EYH_STACK (size_t)(6 * 1024 * 1024)

void eyh_enter(eyh_trap *t, eye_rt *rt) {
	t->up = eyh_top;
	t->job = eyj_cur;
	t->rt = rt;
	t->floor = ey_stack_floor;
	char here;
	ey_stack_floor = &here - EYH_STACK;
	eyh_top = t;
}

void eyh_leave(eyh_trap *t) {
	eyh_top = t->up;
	eyj_cur = t->job;
	ey_stack_floor = t->floor;
}

/* A fault in compiled code goes to the call the runner made. */
_Noreturn void eyh_fault(const char *at, const char *msg) {
	if (!eyh_top) {
		fprintf(stderr, "%s: %s\n", at, msg);
		exit(1);
	}
	free(eyh_message);
	size_t n = strlen(at) + strlen(msg) + 3;
	eyh_message = malloc(n);
	if (eyh_message) snprintf(eyh_message, n, "%s: %s", at, msg);
	longjmp(eyh_top->at, 1);
}

const char *eye_fault(eye_rt *rt) {
	(void)rt;
	return eyh_message;
}

/* A callback failed: the runner knows why, and is told when the compiled
 * code has returned. */
static _Noreturn void eyh_unwind(void) {
	free(eyh_message);
	eyh_message = NULL;
	longjmp(eyh_top->at, 1);
}

int64_t eyh_now(void) {
	int64_t ns = 0;
	if (eyh_top && eye_go_now(eyh_top->rt, &ns) != EYE_OK) eyh_unwind();
	return ns;
}

/* ---------------------------------------------------------------- callbacks */

void eyh_site(int site, eye_val *args, int nargs, ey_decoder d, void *out) {
	eye_val r = EYE_NONE;
	eye_status s = eye_go_site(eyh_top->rt, site, args, nargs, &r);
	for (int i = 0; i < nargs; i++) eye_release(args[i]);
	if (s != EYE_OK) eyh_unwind();
	if (d) eyh_dec(r, d, out);
	eye_release(r);
}

/* ---------------------------------------------------------------- opaque values */

/* Skips one saved value (interp/save.go). */
static bool eyh_skip_values(ey_rd *r);

static bool eyh_skip(ey_rd *r) {
	if (r->p >= r->end) return eys_fail(r, "corrupt saved value");
	char tag = (char)*r->p++;
	uint64_t n;
	int64_t i;
	const char *s;
	int64_t len;
	switch (tag) {
	case 'i': case 'c': case 'd': case 'z': case 'r':
		return eys_read_varint(r, &i);
	case 'I': case 's':
		return eys_read_text(r, &s, &len);
	case 'f':
		return eys_read_uvarint(r, &n);
	case 'T': case 'F': case 'u': case 'E':
		return true;
	case 'D': case 't':
		return eys_read_varint(r, &i) && eys_read_varint(r, &i) && eys_read_varint(r, &i);
	case 'W':
		return eys_read_varint(r, &i) && eys_read_text(r, &s, &len);
	case 'L': case 'S': case 'P':
		return eyh_skip_values(r);
	case 'M':
		return eyh_skip_values(r) && eyh_skip_values(r);
	case 'N':
		if (!eys_read_text(r, &s, &len)) return false;
		/* fall through */
	case 'R':
		if (!eys_read_uvarint(r, &n)) return false;
		for (uint64_t k = 0; k < n; k++)
			if (!eys_read_text(r, &s, &len) || !eyh_skip(r)) return false;
		return true;
	case 'V': case 'C':
		return eys_read_text(r, &s, &len) && eyh_skip_values(r);
	case 'e':
		return eys_read_text(r, &s, &len) && eyh_skip(r);
	case 'A':
		return eys_read_text(r, &s, &len) && eys_read_text(r, &s, &len);
	case 'g': {
		if (r->p >= r->end) return eys_fail(r, "corrupt saved value");
		uint8_t flags = *r->p++;
		if ((flags & 2) && !eyh_skip(r)) return false;
		if ((flags & 4) && !eyh_skip(r)) return false;
		return true;
	}
	}
	return eys_fail(r, "corrupt saved value");
}

static bool eyh_skip_values(ey_rd *r) {
	uint64_t n;
	if (!eys_read_uvarint(r, &n)) return false;
	for (uint64_t k = 0; k < n; k++)
		if (!eyh_skip(r)) return false;
	return true;
}

void eys_opaque(ey_buf *b, ey_v s) {
	ey_buf_add(b, (const char *)EY_STR(s)->b, EY_STR(s)->len);
}

bool eys_read_opaque(ey_rd *r, ey_v *out) {
	const uint8_t *start = r->p;
	if (!eyh_skip(r)) return false;
	*out = ey_str_new((const char *)start, (int64_t)(r->p - start), &ey_t_string);
	return true;
}

bool eyh_is_variant(ey_v s, const char *name, int64_t len) {
	ey_rd r = {(const uint8_t *)EY_STR(s)->b, (const uint8_t *)EY_STR(s)->b + EY_STR(s)->len, NULL};
	if (r.p >= r.end || *r.p != 'V') return false;
	const char *got;
	int64_t n;
	uint64_t count;
	if (!eys_read_variant(&r, &got, &n, &count)) return false;
	return n == len && memcmp(got, name, (size_t)len) == 0;
}

void eyh_payload(ey_v s, int index, ey_decoder d, void *out) {
	ey_rd r = {(const uint8_t *)EY_STR(s)->b, (const uint8_t *)EY_STR(s)->b + EY_STR(s)->len, NULL};
	const char *got;
	int64_t n;
	uint64_t count;
	if (!eys_read_variant(&r, &got, &n, &count) || (uint64_t)index >= count) ey_fault("<runner>", "corrupt saved value");
	for (int i = 0; i < index; i++)
		if (!eyh_skip(&r)) ey_fault("<runner>", "corrupt saved value");
	if (!d(&r, out)) ey_fault("<runner>", r.err ? r.err : "corrupt saved value");
}

void eyh_field(ey_v s, const char *name, int64_t len, ey_decoder d, void *out) {
	ey_rd r = {(const uint8_t *)EY_STR(s)->b, (const uint8_t *)EY_STR(s)->b + EY_STR(s)->len, NULL};
	uint64_t n;
	if (!eys_read_record(&r, &n)) ey_fault("<runner>", "corrupt saved value");
	for (uint64_t i = 0; i < n; i++) {
		const char *got;
		int64_t k;
		if (!eys_read_text(&r, &got, &k)) ey_fault("<runner>", "corrupt saved value");
		if (k == len && memcmp(got, name, (size_t)len) == 0) {
			if (!d(&r, out)) ey_fault("<runner>", r.err ? r.err : "corrupt saved value");
			return;
		}
		if (!eyh_skip(&r)) ey_fault("<runner>", "corrupt saved value");
	}
	ey_fault("<runner>", "a saved record lacks a field its type has");
}

/* ---------------------------------------------------------------- frames */

/* A first frame is 'A' and the arguments' encodings, one after another; any
 * other is 'S' and the functions' segments, from the outside in, each its
 * length and its bytes. */

eye_status eye_job_frame(eye_rt *rt, int32_t job, const eye_val *args, int32_t nargs, eye_val *frame) {
	(void)rt;
	(void)job;
	ey_buf b = {0};
	ey_buf_char(&b, 'A');
	for (int i = 0; i < nargs; i++) ey_buf_add(&b, (const char *)EYH(args[i])->b, (int64_t)EYH(args[i])->n);
	*frame = eyh_bytes(b.b, (size_t)b.len);
	free(b.b);
	return EYE_OK;
}

void eyj_begin(eyj *j, eye_val frame, eye_val resumed) {
	memset(j, 0, sizeof *j);
	eyh_val *f = EYH(frame);
	if (f->n > 0 && f->b[0] == 'S') {
		j->resuming = true;
		j->seg = f->b + 1;
		j->end = f->b + f->n;
	} else {
		j->first = (ey_rd){f->b + (f->n > 0 ? 1 : 0), f->b + f->n, NULL};
	}
	j->resumed = resumed;
	eyj_cur = j;
}

void eyj_arg(eyj *j, ey_decoder d, void *out) {
	if (!d(&j->first, out)) ey_fault("<runner>", j->first.err ? j->first.err : "corrupt saved value");
}

int eyj_next(ey_rd *r, uint64_t code) {
	eyj *j = eyj_cur;
	ey_rd seg = {j->seg, j->end, NULL};
	uint64_t n, got, state;
	if (!eys_read_uvarint(&seg, &n) || n > (uint64_t)(seg.end - seg.p)) ey_fault("<runner>", "a run's saved frame is corrupt");
	r->p = seg.p;
	r->end = seg.p + n;
	r->err = NULL;
	j->seg = r->end;
	if (!eys_read_uvarint(r, &got) || !eys_read_uvarint(r, &state)) ey_fault("<runner>", "a run's saved frame is corrupt");
	if (got != (code & 0x7fffffffffffffff))
		ey_fault("<runner>", "this run was saved by other code: the job's code changed while it waited, so it can't resume here");
	return (int)state;
}

void eyj_stop(int site, eye_val *args, int nargs) {
	eyj *j = eyj_cur;
	j->stopping = true;
	j->site = site;
	j->nargs = nargs;
	j->args = nargs ? malloc(sizeof(eye_val) * (size_t)nargs) : NULL;
	if (nargs) memcpy(j->args, args, sizeof(eye_val) * (size_t)nargs);
}

ey_buf *eyj_save(uint64_t code, int state) {
	eyj *j = eyj_cur;
	if (j->nsegs == j->cap) {
		j->cap = j->cap ? j->cap * 2 : 8;
		j->segs = realloc(j->segs, sizeof(ey_buf) * (size_t)j->cap);
		if (!j->segs) abort();
	}
	ey_buf *b = &j->segs[j->nsegs++];
	memset(b, 0, sizeof *b);
	eys_uvarint(b, code & 0x7fffffffffffffff);
	eys_uvarint(b, (uint64_t)state);
	return b;
}

void eyj_put(ey_buf *b, ey_encoder e, const void *v, size_t size) {
	const char *c = v;
	bool zero = true;
	for (size_t i = 0; i < size && zero; i++) zero = c[i] == 0;
	ey_buf_char(b, zero ? 0 : 1);
	if (!zero) e(b, v);
}

bool eyj_get(ey_rd *r, ey_decoder d, void *out) {
	if (r->p >= r->end) return eys_fail(r, "a run's saved frame is corrupt");
	if (*r->p++ == 0) return true;
	return d(r, out);
}

void eyj_resumed(ey_decoder d, void *out) {
	eyj *j = eyj_cur;
	j->resuming = false;
	if (d) {
		if (j->resumed == EYE_NONE) ey_fault("<runner>", "a step gave nothing to resume from");
		eyh_dec(j->resumed, d, out);
	}
}

eye_val eyj_frame(eyj *j) {
	ey_buf b = {0};
	ey_buf_char(&b, 'S');
	for (int i = j->nsegs - 1; i >= 0; i--) {
		eys_uvarint(&b, (uint64_t)j->segs[i].len);
		ey_buf_add(&b, j->segs[i].b, j->segs[i].len);
	}
	eye_val f = eyh_bytes(b.b, (size_t)b.len);
	free(b.b);
	/* The frame holds the step's arguments until the next call on it. */
	EYH(f)->args = j->args;
	EYH(f)->nargs = j->nargs;
	j->args = NULL;
	j->nargs = 0;
	return f;
}

void eyj_step(eyj *j, eye_val frame, eye_step *step) {
	memset(step, 0, sizeof *step);
	step->kind = EYE_STEP_SITE;
	step->site = j->site;
	step->nargs = EYH(frame)->nargs;
	step->args = EYH(frame)->args;
}

void eyj_end(eyj *j) {
	for (int i = 0; i < j->nsegs; i++) free(j->segs[i].b);
	free(j->segs);
	j->segs = NULL;
	j->nsegs = 0;
	free(j->args);
	j->args = NULL;
}

/* main and the tests stay the interpreter's for now: has_main is 0 and no
 * test is listed, so the runner doesn't call these. */
eye_status eye_run_main(eye_rt *rt, eye_val args, eye_val *result) {
	(void)rt;
	(void)args;
	(void)result;
	return EYE_FAULT;
}

eye_status eye_run_test(eye_rt *rt, int32_t test) {
	(void)rt;
	(void)test;
	return EYE_FAULT;
}

int eye_main(int argc, char **argv) {
	(void)argc;
	(void)argv;
	return 1;
}

#endif
