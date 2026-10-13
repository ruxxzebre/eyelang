/* Recursion too deep is a fault, as in the interpreter (interp/eval.go's
 * maxDepth), not a crash: a test that recurses too deeply fails and the
 * others run, and main ends with the fault.
 *
 * The interpreter counts calls and faults past 100,000. Counting every
 * call costs native code too much (fib took 40% longer), so native code
 * measures the stack instead: the program runs on a thread whose stack is
 * EY_STACK_SIZE, reserved, not committed, and each function that can call
 * itself, directly or through others, checks at its start that the stack
 * has room left (EY_STACK_CHECK: a load and a comparison). The stack holds
 * 100,000 calls with frames up to 2.6 KB, so a recursion the interpreter
 * allows runs; one deeper than the stack holds faults with the
 * interpreter's message. Between the two, a recursion of small frames may
 * go deeper natively than the interpreter lets it. */
#include <stdlib.h>

#ifdef _WIN32
#include <process.h>
#else
#include <pthread.h>
#endif

#include "eye.h"

#define EY_STACK_SIZE ((size_t)256 << 20)
/* what the stack keeps past the floor: room for the runtime's functions,
 * and for the calls of functions that don't check, below the last that
 * did */
#define EY_STACK_MARGIN ((size_t)1 << 20)

char *ey_stack_floor;

_Noreturn void ey_too_deep(const char *at, const char *name) {
	ey_faultf(at, "recursion too deep: %s was called with %d calls already unfinished; recurse less deeply, or use a loop", name,
	          100000);
}

typedef struct stack_call {
	int (*run)(void *);
	void *arg;
	int code;
} stack_call;

/* sets the floor for a stack of size bytes, from where it starts */
static void stack_from(size_t size) {
	char here;
	ey_stack_floor = &here - size + EY_STACK_MARGIN;
}

#ifdef _WIN32
#define STACK_RESERVE 0x00010000 /* STACK_SIZE_PARAM_IS_A_RESERVATION */

static unsigned __stdcall stack_main(void *p) {
	stack_call *c = p;
	stack_from(EY_STACK_SIZE - 64 * 1024);
	exit(c->run(c->arg)); /* the process ends with the program */
}

int ey_on_big_stack(int (*run)(void *), void *arg) {
	static stack_call c;
	c.run = run;
	c.arg = arg;
	if (_beginthreadex(NULL, (unsigned)EY_STACK_SIZE, stack_main, &c, STACK_RESERVE, NULL) != 0) {
		_endthreadex(0); /* the process goes on until the program's thread exits */
	}
	stack_from(EY_STACK_MARGIN + 512 * 1024); /* the main thread's: 1 MB */
	return run(arg);
}
#else
static void *stack_main(void *p) {
	stack_call *c = p;
	stack_from(EY_STACK_SIZE - 64 * 1024);
	c->code = c->run(c->arg);
	return NULL;
}

int ey_on_big_stack(int (*run)(void *), void *arg) {
	stack_call c = {run, arg, 0};
	pthread_attr_t a;
	pthread_t t;
	bool started = false;
	if (pthread_attr_init(&a) == 0) {
		started = pthread_attr_setstacksize(&a, EY_STACK_SIZE) == 0 && pthread_create(&t, &a, stack_main, &c) == 0;
		pthread_attr_destroy(&a);
	}
	if (!started) {
		/* no room for the stack: the main thread's, which has 8 MB on
		 * Linux and macOS unless told otherwise */
		stack_from(EY_STACK_MARGIN + 6 * 1024 * 1024);
		return run(arg);
	}
	pthread_join(t, NULL);
	return c.code;
}
#endif
