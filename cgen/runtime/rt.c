/* Allocation, reference counts, faults, output, and running a program's
 * main or its tests. */
#include <setjmp.h>
#include <stdarg.h>
#include <stdio.h>
#include <stdlib.h>
#include <time.h>

#include "eye.h"
#include "builtins.h"

#define SCALAR(id, k) const ey_type id = {k, 0, NULL, NULL, NULL, NULL}
SCALAR(ey_t_unit, EY_UNIT);
SCALAR(ey_t_bool, EY_BOOL);
SCALAR(ey_t_float, EY_FLOAT);
SCALAR(ey_t_char, EY_CHAR);
SCALAR(ey_t_byte, EY_BYTE);
SCALAR(ey_t_int32, EY_INT32);
SCALAR(ey_t_int64, EY_INT64);
SCALAR(ey_t_uint32, EY_UINT32);
SCALAR(ey_t_uint64, EY_UINT64);
SCALAR(ey_t_duration, EY_DURATION);
SCALAR(ey_t_size, EY_SIZE);
SCALAR(ey_t_date, EY_DATE);
SCALAR(ey_t_time, EY_TIME);
SCALAR(ey_t_never, EY_NEVER);
SCALAR(ey_t_int, EY_INT);
SCALAR(ey_t_string, EY_STRING);
SCALAR(ey_t_bytes, EY_BYTES);
SCALAR(ey_t_iter, EY_ITER);
SCALAR(ey_t_loop, EY_LOOP);
SCALAR(ey_t_big, EY_BIG);
SCALAR(ey_t_instant, EY_INSTANT);

/* ---------------------------------------------------------------- memory */

static _Noreturn void out_of_memory(void) {
	fputs("out of memory\n", stderr);
	exit(2);
}

void *ey_alloc(size_t size, const ey_type *ty) {
	ey_obj *o = malloc(size);
	if (!o) out_of_memory();
	o->rc = 1;
	o->ty = ty;
	return o;
}

static void drop_as(const ey_type *t, ey_v v) {
	if (ey_counted(t)) ey_drop(v);
}

void ey_free(ey_obj *o) {
	const ey_type *t = o->ty;
	switch (t->kind) {
	case EY_TUPLE:
	case EY_RECORD:
	case EY_FN: {
		ey_v *f = t->kind == EY_FN ? ((ey_fn *)o)->cap : ((ey_agg *)o)->f;
		uint32_t n = t->kind == EY_FN ? ((ey_fn *)o)->n : ((ey_agg *)o)->n;
		for (uint32_t i = 0; i < n && i < t->n; i++) drop_as(t->args[i], f[i]);
		break;
	}
	case EY_VARIANT: {
		ey_agg *a = (ey_agg *)o;
		const ey_variant *v = &t->variants[a->tag];
		for (uint32_t i = 0; i < a->n && i < v->n; i++) drop_as(v->fields[i], a->f[i]);
		break;
	}
	case EY_RANGE: {
		ey_agg *a = (ey_agg *)o;
		drop_as(t->args[0], a->f[0]);
		drop_as(t->args[0], a->f[1]);
		break;
	}
	case EY_DYN:
		drop_as(t->args[0], ((ey_dyn *)o)->val);
		break;
	case EY_LIST: {
		ey_list *l = (ey_list *)o;
		if (ey_counted(t->args[0]))
			for (int64_t i = 0; i < l->len; i++) ey_drop(l->items[i]);
		free(l->items);
		break;
	}
	case EY_SET:
	case EY_MAP: {
		ey_map *m = (ey_map *)o;
		if (ey_counted(t->args[0]))
			for (int64_t i = 0; i < m->len; i++) ey_drop(m->keys[i]);
		if (m->vals && t->n > 1 && ey_counted(t->args[1]))
			for (int64_t i = 0; i < m->len; i++) ey_drop(m->vals[i]);
		free(m->keys);
		free(m->vals);
		free(m->index);
		break;
	}
	case EY_ITER:
		ey_lazy_free(o);
		return;
	case EY_LOOP:
		ey_loop_free(o);
		return;
	default:
		break;
	}
	free(o);
}

/* ---------------------------------------------------------------- buffers */

void ey_buf_add(ey_buf *b, const char *s, int64_t n) {
	if (b->len + n + 1 > b->cap) {
		int64_t c = b->cap ? b->cap * 2 : 64;
		while (c < b->len + n + 1) c *= 2;
		b->b = realloc(b->b, (size_t)c);
		if (!b->b) out_of_memory();
		b->cap = c;
	}
	memcpy(b->b + b->len, s, (size_t)n);
	b->len += n;
	b->b[b->len] = 0;
}

void ey_buf_str(ey_buf *b, const char *s) { ey_buf_add(b, s, (int64_t)strlen(s)); }
void ey_buf_char(ey_buf *b, char c) { ey_buf_add(b, &c, 1); }

void ey_buf_rune(ey_buf *b, uint32_t r) {
	char s[4];
	if (r < 0x80) {
		s[0] = (char)r;
		ey_buf_add(b, s, 1);
	} else if (r < 0x800) {
		s[0] = (char)(0xc0 | (r >> 6));
		s[1] = (char)(0x80 | (r & 0x3f));
		ey_buf_add(b, s, 2);
	} else if (r < 0x10000) {
		s[0] = (char)(0xe0 | (r >> 12));
		s[1] = (char)(0x80 | ((r >> 6) & 0x3f));
		s[2] = (char)(0x80 | (r & 0x3f));
		ey_buf_add(b, s, 3);
	} else {
		s[0] = (char)(0xf0 | (r >> 18));
		s[1] = (char)(0x80 | ((r >> 12) & 0x3f));
		s[2] = (char)(0x80 | ((r >> 6) & 0x3f));
		s[3] = (char)(0x80 | (r & 0x3f));
		ey_buf_add(b, s, 4);
	}
}

void ey_buf_printf(ey_buf *b, const char *fmt, ...) {
	char small[256];
	va_list ap;
	va_start(ap, fmt);
	int n = vsnprintf(small, sizeof small, fmt, ap);
	va_end(ap);
	if (n < (int)sizeof small) {
		ey_buf_add(b, small, n);
		return;
	}
	char *big = malloc((size_t)n + 1);
	if (!big) out_of_memory();
	va_start(ap, fmt);
	vsnprintf(big, (size_t)n + 1, fmt, ap);
	va_end(ap);
	ey_buf_add(b, big, n);
	free(big);
}

ey_v ey_buf_take(ey_buf *b) {
	ey_v s = ey_str_new(b->b ? b->b : "", b->len, &ey_t_string);
	free(b->b);
	b->b = NULL;
	b->len = b->cap = 0;
	return s;
}

/* ---------------------------------------------------------------- faults and output */

static bool testing;         /* running tests, not main */
static jmp_buf test_fault;   /* where a fault in a test goes */
static ey_buf test_log;      /* what the running test printed and logged */
static ey_buf test_failure;  /* why it failed */

_Noreturn void ey_fault(const char *at, const char *msg) {
	if (testing) {
		test_failure.len = 0;
		ey_buf_str(&test_failure, at);
		ey_buf_str(&test_failure, ": ");
		ey_buf_str(&test_failure, msg);
		longjmp(test_fault, 1);
	}
	fflush(stdout);
	fprintf(stderr, "%s: %s\n", at, msg);
	exit(1);
}

_Noreturn void ey_faultf(const char *at, const char *fmt, ...) {
	char msg[1024];
	va_list ap;
	va_start(ap, fmt);
	vsnprintf(msg, sizeof msg, fmt, ap);
	va_end(ap);
	ey_fault(at, msg);
}

_Noreturn void ey_unsupported(const char *at, const char *what) {
	ey_faultf(at, "the C back end doesn't support %s yet", what);
}

_Noreturn void ey_expect_failed(const char *at, ey_v src, ey_v lsrc, const ey_type *lt, ey_v l) {
	ey_buf b = {0};
	ey_buf_str(&b, "expected ");
	ey_buf_add(&b, EY_STR(src)->b, EY_STR(src)->len);
	if (lt) {
		ey_buf_str(&b, ", but ");
		ey_buf_add(&b, EY_STR(lsrc)->b, EY_STR(lsrc)->len);
		ey_buf_str(&b, " is ");
		ey_show_to(&b, lt, l, true);
	}
	ey_fault(at, b.b);
}

_Noreturn void ey_unreachable(const char *at, ey_v why) {
	ey_buf b = {0};
	ey_buf_str(&b, "unreachable: ");
	ey_buf_add(&b, EY_STR(why)->b, EY_STR(why)->len);
	ey_fault(at, b.b);
}

void ey_eprintln(ey_v s) {
	fflush(stdout);
	fwrite(EY_STR(s)->b, 1, (size_t)EY_STR(s)->len, stderr);
	fputc('\n', stderr);
}

ey_v eyb_env_get(ey_v name, const ey_type *opt) {
	const char *v = getenv(EY_STR(name)->b);
	if (!v) return ey_none(opt);
	return ey_some(opt, ey_str_c(v));
}

static void write_parts(ey_buf *b, int n, const ey_v *parts) {
	for (int i = 0; i < n; i++) {
		if (i > 0) ey_buf_char(b, ' ');
		ey_buf_add(b, EY_STR(parts[i])->b, EY_STR(parts[i])->len);
	}
	ey_buf_char(b, '\n');
}

void ey_print(int n, const ey_v *parts) {
	if (testing) {
		write_parts(&test_log, n, parts);
		return;
	}
	ey_buf b = {0};
	write_parts(&b, n, parts);
	fwrite(b.b, 1, (size_t)b.len, stdout);
	free(b.b);
}

void ey_log(const char *level, int n, const ey_v *parts) {
	ey_buf b = {0};
	ey_buf_str(&b, level);
	ey_buf_str(&b, ": ");
	write_parts(&b, n, parts);
	if (testing)
		ey_buf_add(&test_log, b.b, b.len);
	else
		fwrite(b.b, 1, (size_t)b.len, stderr);
	free(b.b);
}

/* ---------------------------------------------------------------- running */

static int g_argc;
static char **g_argv;

ey_v ey_args(const ey_type *list) {
	ey_v *items = malloc(sizeof(ey_v) * (size_t)(g_argc > 0 ? g_argc : 1));
	if (!items) out_of_memory();
	for (int i = 0; i < g_argc; i++) items[i] = ey_str_c(g_argv[i]);
	return ey_list_take(list, g_argc, items);
}

static int64_t now_ns(void) {
	struct timespec ts;
	timespec_get(&ts, TIME_UTC);
	return (int64_t)ts.tv_sec * 1000000000 + ts.tv_nsec;
}

ey_v ey_now(void) { return ey_instant_new(now_ns(), NULL); }

/* fmt_frac writes v / 10^prec with the zeros at its end left out, as Go's
 * time.Duration does. */
static void fmt_frac(ey_buf *b, uint64_t v, int prec) {
	char digits[32];
	int n = 0;
	bool print = false;
	for (int i = 0; i < prec; i++) {
		int digit = (int)(v % 10);
		print = print || digit != 0;
		if (print) digits[n++] = (char)('0' + digit);
		v /= 10;
	}
	char whole[32];
	int w = snprintf(whole, sizeof whole, "%llu", (unsigned long long)v);
	ey_buf_add(b, whole, w);
	if (n > 0) {
		ey_buf_char(b, '.');
		while (n > 0) ey_buf_char(b, digits[--n]);
	}
}

/* write_duration writes d, rounded to the microsecond, as Go's
 * time.Duration prints it, which eye test does: 22µs, 6.41ms, 1.5s. */
static void write_duration(ey_buf *b, int64_t ns) {
	uint64_t u = (uint64_t)((ns + 500) / 1000 * 1000);
	if (u == 0) {
		ey_buf_str(b, "0s");
	} else if (u < 1000000) {
		fmt_frac(b, u, 3);
		ey_buf_str(b, "µs");
	} else if (u < 1000000000) {
		fmt_frac(b, u, 6);
		ey_buf_str(b, "ms");
	} else {
		uint64_t secs = u / 1000000000, frac = u % 1000000000;
		if (secs >= 3600) ey_buf_printf(b, "%lluh", (unsigned long long)(secs / 3600));
		if (secs >= 60) ey_buf_printf(b, "%llum", (unsigned long long)(secs / 60 % 60));
		fmt_frac(b, (secs % 60) * 1000000000 + frac, 9);
		ey_buf_char(b, 's');
	}
}

static void write_indented(ey_buf *out, const char *s, int64_t n) {
	while (n > 0 && s[n - 1] == '\n') n--;
	int64_t start = 0;
	for (int64_t i = 0; i <= n; i++) {
		if (i == n || s[i] == '\n') {
			ey_buf_str(out, "      ");
			ey_buf_add(out, s + start, i - start);
			ey_buf_char(out, '\n');
			start = i + 1;
		}
	}
}

static int run_tests(const ey_program *p, int argc, char **argv) {
	const char *filter = "";
	bool verbose = false;
	for (int i = 1; i < argc; i++) {
		if (strcmp(argv[i], "-v") == 0) {
			verbose = true;
		} else if ((strcmp(argv[i], "--run") == 0 || strcmp(argv[i], "-run") == 0) && i + 1 < argc) {
			filter = argv[++i];
		} else {
			fprintf(stderr, "usage: %s [--run text] [-v]\n\nRuns the program's test blocks.\n", argv[0]);
			return 2;
		}
	}
	testing = true;
	ey_sandbox(true);
	int ran = 0, failed = 0;
	int64_t total = 0;
	ey_buf out = {0};
	for (int i = 0; i < p->ntests; i++) {
		const ey_test *t = &p->tests[i];
		if (!strstr(t->name, filter)) continue;
		ran++;
		test_log.len = 0;
		test_failure.len = 0;
		int64_t start = now_ns();
		volatile bool ok = true;
		if (setjmp(test_fault) == 0)
			t->run();
		else
			ok = false;
		int64_t took = now_ns() - start;
		total += took;
		ey_sandbox_end();
		ey_buf_str(&out, ok ? "ok    " : "FAIL  ");
		ey_buf_str(&out, t->name);
		ey_buf_str(&out, " (");
		write_duration(&out, took);
		ey_buf_str(&out, ")\n");
		if (!ok) {
			failed++;
			write_indented(&out, test_failure.b, test_failure.len);
			ey_buf_char(&out, '\n'); /* as eye test writes it */
		}
		if ((!ok || verbose) && test_log.len > 0) write_indented(&out, test_log.b, test_log.len);
		fwrite(out.b, 1, (size_t)out.len, stdout);
		fflush(stdout);
		out.len = 0;
	}
	if (ran == 0) {
		ey_buf_str(&out, "no tests to run\n");
	} else if (failed > 0) {
		ey_buf_printf(&out, "%d of %d tests failed\n", failed, ran);
	} else {
		ey_buf_printf(&out, "%d tests passed (", ran);
		write_duration(&out, total);
		ey_buf_str(&out, ")\n");
	}
	fwrite(out.b, 1, (size_t)out.len, stdout);
	free(out.b);
	return failed > 0 ? 1 : 0;
}

int ey_start(const ey_program *p, int argc, char **argv) {
	g_argc = argc;
	g_argv = argv;
	ey_zones(p->zones, p->nzones);
	if (!p->main) return run_tests(p, argc, argv);
	p->main();
	fflush(stdout);
	return 0;
}
