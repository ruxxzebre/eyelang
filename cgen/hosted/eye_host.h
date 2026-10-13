/* The interface between the Go runner and a compiled Eyelang program
 * (compilation.md, "Hosting compiled programs (7.6)").
 *
 * The compiled program defines the entry points; the runner, the program's
 * main under `eye build --runner`, calls them through cgo. The runner
 * defines the callbacks, exported from Go, and compiled code calls them.
 *
 * Values cross as opaque handles, eye_val, whose meaning is the compiled
 * program's (7.13's layouts). The runner never looks inside one: it reads a
 * value with eye_encode and makes one with eye_decode, in the encoding the
 * interpreter saves values in (interp/save.go), so a journal or a Postgres
 * store moves between interpreted and compiled code. */
#ifndef EYE_HOST_H
#define EYE_HOST_H

#include <stddef.h>
#include <stdint.h>

/* EYE_HOST_VERSION changes when this interface does; eye_program says which
 * version the program was compiled for, and the runner refuses another. */
#define EYE_HOST_VERSION 1

/* The runner's side of one call: its run, its transaction and its frame.
 * Compiled code passes it back in every callback and never looks inside. */
typedef struct eye_rt eye_rt;

/* A program value. EYE_NONE is no value: a job's first step resumes from
 * nothing. */
typedef uint64_t eye_val;
#define EYE_NONE ((eye_val)0)

/* Bytes the caller owns for the length of the call. */
typedef struct {
	const uint8_t *p;
	size_t n;
} eye_bytes;

/* Bytes the callee allocated with malloc; the caller frees them. */
typedef struct {
	uint8_t *p;
	size_t n;
} eye_buf;

typedef enum {
	EYE_OK = 0,      /* done, with a result */
	EYE_FAULT = 1,   /* stopped; eye_fault says why */
	EYE_WAITING = 2, /* a job reached a `!` step: the runner takes it */
} eye_status;

/* ---- what the program declares
 *
 * Every table is by index, and every type by its index in eye_program_info.types.
 * Names are as the runner names them: a declaration of the program's own
 * module by its name ("Counter"), one of an imported module with the
 * module's path ("lab.notify.Message"). */

typedef struct {
	const char *name;
	int32_t nparams;
	const int32_t *param_types;
	int32_t result_type;
} eye_method_info;

typedef struct {
	const char *name;
	int32_t id_type;    /* the identity's type */
	int32_t state_type; /* the state, as the store keeps it: a record of its fields */
	int32_t nmethods;
	const eye_method_info *methods;
} eye_entity_info;

typedef struct {
	const char *name;
	int32_t nparams;
	const int32_t *param_types;
	int32_t frame_type; /* the frame a run is saved as between steps */
	int32_t result_type;
} eye_job_info;

typedef struct eye_program_info {
	int32_t version; /* EYE_HOST_VERSION */
	int32_t ntypes;
	const char *const *types; /* each saved type, named as `eye check --ir` prints it */
	int32_t nentities;
	const eye_entity_info *entities;
	int32_t njobs;
	const eye_job_info *jobs;
	int32_t ntests;
	const char *const *tests;
	int32_t nhosts;
	const char *const *hosts; /* the host functions eye_go_host calls, by name */
} eye_program_info;

/* ---- a job's step
 *
 * A job runs from step to step: eye_step_job returns at each `!` with the
 * step to take. The runner takes it, records it, and calls eye_step_job
 * again with what it gave. So a waiting run holds no C stack and no thread,
 * only its encoded frame. */

typedef enum {
	EYE_STEP_SLEEP = 1, /* sleep!(…): sleep_ns */
	EYE_STEP_CALL = 2,  /* Entity(id).method!(args) */
	EYE_STEP_HOST = 3,  /* a host function that waits or reaches outside: fn(args) */
} eye_step_kind;

typedef struct eye_step {
	eye_step_kind kind;
	int32_t result_type; /* the type of what the step gives back */
	int64_t sleep_ns;    /* EYE_STEP_SLEEP */
	int32_t entity;      /* EYE_STEP_CALL */
	int32_t method;      /* EYE_STEP_CALL */
	eye_val id;          /* EYE_STEP_CALL: the entity's identity */
	int32_t fn;          /* EYE_STEP_HOST: an index into eye_program_info.hosts */
	int32_t nargs;
	const eye_val *args; /* owned by the frame, until the next call on it */
} eye_step;

/* ---- entry points: the compiled program, called by the runner */

const eye_program_info *eye_program(void);

/* Runs main, or the tests of a --test build, with these arguments, as a C
 * main would, and returns the exit status. Built since 7.6's groundwork. */
int eye_main(int argc, char **argv);

eye_status eye_run_main(eye_rt *rt, eye_val args);
eye_status eye_run_test(eye_rt *rt, int32_t test);

/* Runs an entity's method on its state, in the runner's transaction.
 * *state is replaced by the new state: in place when the runner held the
 * only reference. The result is the method's. On EYE_FAULT, *state is the
 * state it was given, still the runner's to release; so is *frame for
 * eye_step_job. */
eye_status eye_call_method(eye_rt *rt, int32_t entity, int32_t method, eye_val *state,
                           const eye_val *args, int32_t nargs, eye_val *result);

/* Makes a job's first frame from its arguments. */
eye_status eye_job_frame(eye_rt *rt, int32_t job, const eye_val *args, int32_t nargs, eye_val *frame);

/* Runs a job from its frame to its next `!` or to its end. resumed is what
 * the step it waited on gave, or EYE_NONE the first time. On EYE_WAITING,
 * *frame is the new frame and *step the step to take; on EYE_OK, *result is
 * the job's result. */
eye_status eye_step_job(eye_rt *rt, int32_t job, eye_val *frame, eye_val resumed,
                        eye_step *step, eye_val *result);

/* A value's encoding, and the value an encoding holds, for type. */
eye_status eye_encode(eye_rt *rt, int32_t type, eye_val v, eye_buf *out);
eye_status eye_decode(eye_rt *rt, int32_t type, eye_bytes in, eye_val *out);

void eye_retain(eye_val v);
void eye_release(eye_val v);

/* Why the last call on rt returned EYE_FAULT. */
const char *eye_fault(eye_rt *rt);

/* ---- callbacks: the runner, called by compiled code
 *
 * Entity calls, reads, starts and publishing finish within the transaction,
 * so they are calls rather than steps. Not hosted yet: they come with the
 * rest of 7.6. */

eye_status eye_go_entity_call(eye_rt *rt, int32_t entity, eye_val id, int32_t method,
                              const eye_val *args, int32_t nargs, eye_val *result);
eye_status eye_go_entity_read(eye_rt *rt, int32_t entity, eye_val id, eye_val *snapshot);
eye_status eye_go_start(eye_rt *rt, int32_t job, const eye_val *args, int32_t nargs,
                        eye_val key, eye_val *run);
eye_status eye_go_publish(eye_rt *rt, int32_t event, eye_val value);
/* A host function that doesn't wait (env.get, the clock, random, log): it
 * runs at once. One that waits is a step, which eye_step_job returns. */
eye_status eye_go_host(eye_rt *rt, int32_t fn, const eye_val *args, int32_t nargs,
                       eye_val *result);

#endif
