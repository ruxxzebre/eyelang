/* Host functions the Go runner answers (ROADMAP 7.6, 7.9): http.get, for
 * which the C runtime has no HTTP client or TLS. Under B′ the runner is the
 * program's main and links the program's C in (compilation.md, "Where the
 * runtime lives"); it sets a hook with ey_set_host, and compiled code calls
 * it through ey_host_call. A call crosses as saved bytes (save.c): the
 * arguments given, as a record by parameter name, and the answer, the
 * function's result, which the caller decodes as its type. The runner
 * fills in the arguments left out, as the interpreter binds a call.
 *
 * A test binary can be answered over two file descriptors instead, which
 * EYE_HOST_FDS names ("R,W": answers are read from R, requests written to
 * W), as cgen's tests answer it with the interpreter's host functions. A
 * request is the name's length (4 bytes, little-endian), the name, the
 * arguments' length (8 bytes) and the arguments; an answer is a byte, 0
 * for a result and 1 for a fault, its length (8 bytes) and its bytes.
 *
 * Without either, a call is a fault that says the function needs the
 * runner. Every static name here starts with host_, since at -O2 the
 * runtime is one translation unit. */
#include <stdio.h>
#include <stdlib.h>
#ifndef _WIN32
#include <unistd.h>
#endif

#include "eye.h"
#include "builtins.h"

static ey_host_fn host_hook;

void ey_set_host(ey_host_fn f) { host_hook = f; }

#ifndef _WIN32
static int host_fd_in = -2, host_fd_out = -2; /* -2: not looked for yet; -1: none */

static bool host_fds(void) {
	if (host_fd_in == -2) {
		host_fd_in = host_fd_out = -1;
		const char *s = getenv("EYE_HOST_FDS");
		int r, w;
		if (ey_host_fds && s && sscanf(s, "%d,%d", &r, &w) == 2 && r >= 0 && w >= 0) host_fd_in = r, host_fd_out = w;
	}
	return host_fd_in >= 0;
}

static bool host_write(const void *p, size_t n) {
	const char *c = p;
	while (n > 0) {
		ssize_t k = write(host_fd_out, c, n);
		if (k <= 0) return false;
		c += k;
		n -= (size_t)k;
	}
	return true;
}

static bool host_read(void *p, size_t n) {
	char *c = p;
	while (n > 0) {
		ssize_t k = read(host_fd_in, c, n);
		if (k <= 0) return false;
		c += k;
		n -= (size_t)k;
	}
	return true;
}

static void host_le(uint8_t *b, uint64_t v, int n) {
	for (int i = 0; i < n; i++) b[i] = (uint8_t)(v >> (8 * i));
}

static bool host_pipe(const char *name, const char *args, int64_t n, ey_buf *answer, ey_buf *err) {
	uint8_t head[8];
	size_t len = strlen(name);
	host_le(head, len, 4);
	if (!host_write(head, 4) || !host_write(name, len)) goto broken;
	host_le(head, (uint64_t)n, 8);
	if (!host_write(head, 8) || !host_write(args, (size_t)n)) goto broken;
	uint8_t status;
	if (!host_read(&status, 1) || !host_read(head, 8)) goto broken;
	uint64_t size = 0;
	for (int i = 0; i < 8; i++) size |= (uint64_t)head[i] << (8 * i);
	ey_buf *to = status == 0 ? answer : err;
	char chunk[4096];
	while (size > 0) {
		size_t k = size < sizeof chunk ? (size_t)size : sizeof chunk;
		if (!host_read(chunk, k)) goto broken;
		ey_buf_add(to, chunk, (int64_t)k);
		size -= k;
	}
	return status == 0;
broken:
	ey_buf_str(err, "the connection to whatever answers host functions broke");
	return false;
}
#else
static bool host_fds(void) { return false; }
static bool host_pipe(const char *name, const char *args, int64_t n, ey_buf *answer, ey_buf *err) {
	(void)name, (void)args, (void)n, (void)answer, (void)err;
	return false;
}
#endif

void ey_host_call(const char *at, const char *name, ey_buf *args, ey_decoder dec, void *out) {
	ey_buf answer = {0}, err = {0};
	bool ok;
	if (host_hook)
		ok = host_hook(name, args->b ? args->b : "", args->len, &answer, &err);
	else if (host_fds())
		ok = host_pipe(name, args->b ? args->b : "", args->len, &answer, &err);
	else
		ey_faultf(at, "%s needs the Go runner, which answers it, and this binary was built without it (ROADMAP 7.6 links it in); `eye run` and `eye test` run this program", name);
	free(args->b);
	args->b = NULL;
	if (!ok) ey_fault(at, err.b ? err.b : "the host function failed"); /* err is left to the fault */
	ey_rd r = {(const uint8_t *)answer.b, (const uint8_t *)answer.b + answer.len, NULL};
	ok = dec(&r, out);
	if (!ok || r.p != r.end) ey_faultf(at, "%s answered with a value of another type: %s", name, r.err ? r.err : "bytes left over");
	free(answer.b);
}
