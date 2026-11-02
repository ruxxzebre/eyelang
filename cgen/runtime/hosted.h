/* What generated code uses when the Go runner hosts the program (ROADMAP
 * 7.6; compilation.md, "Compiled jobs"). hosted.c has the code. */
#ifndef EYE_HOSTED_H
#define EYE_HOSTED_H

#ifdef EY_HOSTED

#include <setjmp.h>

#include "data.h"
#include "eye_host.h"

/* One call from the runner into compiled code: where a fault in it goes. A
 * callback nested inside it may call in again, so these make a stack. */
typedef struct eyh_trap {
	jmp_buf at;
	struct eyh_trap *up;
	struct eyj *job;
	eye_rt *rt;
	char *floor;
} eyh_trap;

void eyh_enter(eyh_trap *t, eye_rt *rt);
void eyh_leave(eyh_trap *t);
#define EYH_TRY(t, rt)         \
	eyh_enter(&(t), (rt));     \
	if (setjmp((t).at)) {      \
		eyh_leave(&(t));       \
		return EYE_FAULT;      \
	}

/* A value's handle: its encoding, which the runner holds between the calls
 * of one step. */
eye_val eyh_enc(ey_encoder e, const void *v);
void eyh_dec(eye_val h, ey_decoder d, void *out);

/* Asks the runner to evaluate a call of the program with these values, and
 * reads what it gives as d says (no d: nothing is read). Releases them. */
void eyh_site(int site, eye_val *args, int nargs, ey_decoder d, void *out);

/* A value that only the runner reads, kept as its encoding: a Ref, a Run, a
 * function the runner runs. */
void eys_opaque(ey_buf *b, ey_v s);
bool eys_read_opaque(ey_rd *r, ey_v *out);
/* Whether an opaque value is of the variant of this saved name, and a field
 * of the variant it is, which the caller then owns. */
bool eyh_is_variant(ey_v s, const char *name, int64_t len);
void eyh_payload(ey_v s, int index, ey_decoder d, void *out);
/* A field of an opaque value that is a record, which the caller then owns. */
void eyh_field(ey_v s, const char *name, int64_t len, ey_decoder d, void *out);

/* ---- a job's run from one step to the next
 *
 * A function that stops saves its live variables as bytes, a segment of its
 * own, and returns; each function it was called from then does the same. So
 * a waiting run is only those bytes. To resume, the job's function is called
 * again: each function, in order from the outside in, reads its segment and
 * jumps to where it stopped. */
typedef struct eyj {
	bool resuming, stopping;
	const uint8_t *seg, *end; /* the next segment to read */
	eye_val resumed;
	/* the step it stopped at */
	int site;
	eye_val *args;
	int nargs;
	/* what was saved, from the inside out */
	ey_buf *segs;
	int nsegs, cap;
	ey_rd first; /* a first frame's arguments */
} eyj;

extern _Thread_local eyj *eyj_cur;

void eyj_begin(eyj *j, eye_val frame, eye_val resumed);
/* What a stopped job gives the runner. */
eye_val eyj_frame(eyj *j);
void eyj_step(eyj *j, eye_val frame, eye_step *step);
void eyj_end(eyj *j);
/* The next of a first frame's arguments. */
void eyj_arg(eyj *j, ey_decoder d, void *out);

/* Reads this function's segment, checked against its code's hash, and gives
 * the place it stopped. */
int eyj_next(ey_rd *r, uint64_t code);
/* Stops at a step: the values are the frame's now. */
void eyj_stop(int site, eye_val *args, int nargs);
/* Starts this function's segment. */
ey_buf *eyj_save(uint64_t code, int state);
/* A live variable, which may not have been set on the way here: all zero
 * bits are saved as nothing, and read back as zero bits. */
void eyj_put(ey_buf *b, ey_encoder e, const void *v, size_t size);
bool eyj_get(ey_rd *r, ey_decoder d, void *out);
/* What the step gave. */
void eyj_resumed(ey_decoder d, void *out);

#endif
#endif
