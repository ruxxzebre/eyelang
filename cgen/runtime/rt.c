/* Allocation, reference counts, faults, output, and running a program's
 * main or its tests. */
#include <setjmp.h>
#include <stdarg.h>
#include <stdio.h>
#include <stdlib.h>
#include <time.h>

#include "eye.h"
#include "builtins.h"

#define SCALAR(id, k, sz) const ey_type id = {.kind = k, .size = sz, .align = sz, .repr = EY_R_SCALAR}
#define WORD(id, k) const ey_type id = {.kind = k, .size = 8, .align = 8, .repr = EY_R_WORD}
SCALAR(ey_t_unit, EY_UNIT, 1);
SCALAR(ey_t_bool, EY_BOOL, 1);
SCALAR(ey_t_float, EY_FLOAT, 8);
SCALAR(ey_t_char, EY_CHAR, 4);
SCALAR(ey_t_byte, EY_BYTE, 1);
SCALAR(ey_t_int32, EY_INT32, 4);
SCALAR(ey_t_int64, EY_INT64, 8);
SCALAR(ey_t_uint32, EY_UINT32, 4);
SCALAR(ey_t_uint64, EY_UINT64, 8);
SCALAR(ey_t_duration, EY_DURATION, 8);
SCALAR(ey_t_size, EY_SIZE, 8);
SCALAR(ey_t_date, EY_DATE, 8);
SCALAR(ey_t_time, EY_TIME, 4);
SCALAR(ey_t_never, EY_NEVER, 1);
WORD(ey_t_int, EY_INT);
WORD(ey_t_string, EY_STRING);
WORD(ey_t_bytes, EY_BYTES);
WORD(ey_t_iter, EY_ITER);
WORD(ey_t_loop, EY_LOOP);
WORD(ey_t_big, EY_BIG);
WORD(ey_t_instant, EY_INSTANT);

/* ---------------------------------------------------------------- words and scalars */

static bool signed_kind(uint32_t k) {
	switch (k) {
	case EY_INT32:
	case EY_INT64:
	case EY_DURATION:
	case EY_SIZE:
	case EY_DATE:
	case EY_TIME:
		return true;
	}
	return false;
}

ey_v ey_load_v(const ey_type *t, const void *p) {
	switch (t->size) {
	case 1:
		return *(const uint8_t *)p;
	case 2:
		return *(const uint16_t *)p;
	case 4:
		if (signed_kind(t->kind)) return (ey_v)(int64_t) * (const int32_t *)p;
		return *(const uint32_t *)p;
	case 8:
		return *(const uint64_t *)p;
	}
	return 0;
}

void ey_store_v(const ey_type *t, void *p, ey_v v) {
	switch (t->size) {
	case 1:
		*(uint8_t *)p = (uint8_t)v;
		break;
	case 2:
		*(uint16_t *)p = (uint16_t)v;
		break;
	case 4:
		*(uint32_t *)p = (uint32_t)v;
		break;
	case 8:
		*(uint64_t *)p = v;
		break;
	}
}

void ey_dup_n(const ey_type *t, void *p, int64_t n) {
	if (!ey_counted(t)) return;
	for (int64_t i = 0; i < n; i++) ey_dup_at(t, (char *)p + i * (int64_t)t->size);
}

void ey_drop_n(const ey_type *t, void *p, int64_t n) {
	if (!ey_counted(t)) return;
	for (int64_t i = 0; i < n; i++) ey_drop_at(t, (char *)p + i * (int64_t)t->size);
}

/* ---------------------------------------------------------------- memory */

_Noreturn void ey_out_of_memory(void) {
	fputs("out of memory\n", stderr);
	exit(2);
}

/* Small blocks come from free lists, one per size class of 8 bytes up to
 * EY_CLASS_MAX, carved from chunks that are taken from malloc and never
 * given back; larger blocks come from malloc. A program runs on one
 * thread (eye.h), so the lists need no lock. The caller says a block's
 * size when it frees it, so a block has no header. */
#define EY_CLASS_MAX 256
#define EY_CHUNK (256 * 1024)

static void *free_lists[EY_CLASS_MAX / 8 + 1];
static char *chunk_at, *chunk_end;

uint64_t ey_allocs, ey_frees;

static void *carve(size_t size) {
	if ((size_t)(chunk_end - chunk_at) < size) {
		/* what is left of the old chunk, less than a block, is lost */
		chunk_at = malloc(EY_CHUNK);
		if (!chunk_at) ey_out_of_memory();
		chunk_end = chunk_at + EY_CHUNK;
	}
	void *p = chunk_at;
	chunk_at += size;
	return p;
}

void *ey_mem_alloc(size_t n) {
	if (n <= EY_CLASS_MAX) {
		size_t c = (n + 7) >> 3;
		void *p = free_lists[c];
		if (p) {
			free_lists[c] = *(void **)p;
			return p;
		}
		return carve(c << 3);
	}
	void *p = malloc(n);
	if (!p) ey_out_of_memory();
	return p;
}

void ey_mem_free(void *p, size_t n) {
	if (n <= EY_CLASS_MAX) {
		size_t c = (n + 7) >> 3;
		*(void **)p = free_lists[c];
		free_lists[c] = p;
		return;
	}
	free(p);
}

void *ey_mem_realloc(void *p, size_t old, size_t n) {
	if (old > EY_CLASS_MAX && n > EY_CLASS_MAX) {
		p = realloc(p, n);
		if (!p) ey_out_of_memory();
		return p;
	}
	if (n <= EY_CLASS_MAX && (old + 7) >> 3 == (n + 7) >> 3) return p;
	void *q = ey_mem_alloc(n);
	memcpy(q, p, old < n ? old : n);
	ey_mem_free(p, old);
	return q;
}

void *ey_alloc(size_t size, const ey_type *ty) {
	ey_obj *o = ey_mem_alloc(size);
	ey_allocs++;
	o->rc = 1;
	o->tag = 0;
	o->ty = ty;
	return o;
}

/* The size of an object as it was allocated, which freeing it needs. */
static size_t obj_size(const ey_obj *o) {
	const ey_type *t = o->ty;
	switch (t->kind) {
	case EY_TUPLE:
	case EY_RECORD:
		return t->objsize;
	case EY_VARIANT:
		return t->variants[o->tag].objsize;
	case EY_FN:
	case EY_DYN:
		return t->objsize;
	case EY_RANGE:
		return sizeof(ey_range_obj);
	case EY_STRING:
	case EY_BYTES: {
		const ey_str *s = (const ey_str *)o;
		return sizeof(ey_str) + (size_t)(s->cap ? s->cap : s->len + 1);
	}
	case EY_BIG:
		return sizeof(ey_big) + ((const ey_big *)o)->n * sizeof(uint32_t);
	case EY_LIST:
		return sizeof(ey_list);
	case EY_SET:
	case EY_MAP:
		return sizeof(ey_map);
	case EY_INSTANT:
		return sizeof(ey_instant);
	default:
		ey_fault("<free>", "an object of no known size");
	}
}

void ey_free_obj(ey_obj *o, size_t size) {
	ey_frees++;
	ey_mem_free(o, size);
}

/* drop_fields drops the n fields of types ts at offsets offs from base */
static void drop_fields(char *base, uint32_t n, const ey_type *const *ts, const uint32_t *offs) {
	for (uint32_t i = 0; i < n; i++) ey_drop_at(ts[i], base + offs[i]);
}

void ey_free(ey_obj *o) {
	const ey_type *t = o->ty;
	switch (t->kind) {
	case EY_TUPLE:
	case EY_RECORD:
	case EY_FN:
		drop_fields((char *)o, t->n, t->args, t->offs);
		break;
	case EY_VARIANT: {
		const ey_variant *v = &t->variants[o->tag];
		drop_fields((char *)o, v->n, v->fields, v->offs);
		break;
	}
	case EY_RANGE: {
		ey_range_obj *r = (ey_range_obj *)o;
		if (ey_counted(t->args[0])) {
			ey_drop(r->start);
			ey_drop(r->end);
		}
		break;
	}
	case EY_DYN:
		ey_drop_at(t->args[0], EY_DYN_VAL(o));
		break;
	case EY_LIST: {
		ey_list *l = (ey_list *)o;
		if (l->owner) {
			ey_drop(l->owner);
			break;
		}
		ey_drop_n(t->args[0], l->items, l->len);
		free(l->items);
		break;
	}
	case EY_SET:
	case EY_MAP: {
		ey_map *m = (ey_map *)o;
		ey_drop_n(t->args[0], m->keys, m->len);
		if (m->vals) ey_drop_n(t->args[1], m->vals, m->len);
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
	ey_free_obj(o, obj_size(o));
}

/* ---------------------------------------------------------------- buffers */

void ey_buf_add(ey_buf *b, const char *s, int64_t n) {
	if (b->len + n + 1 > b->cap) {
		int64_t c = b->cap ? b->cap * 2 : 64;
		while (c < b->len + n + 1) c *= 2;
		b->b = realloc(b->b, (size_t)c);
		if (!b->b) ey_out_of_memory();
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

/* Integers in decimal, without snprintf, which is slow in musl. */
static const char digit_pairs[] = "0001020304050607080910111213141516171819202122232425262728293031323334353637383940414243444546474849"
				  "5051525354555657585960616263646566676869707172737475767778798081828384858687888990919293949596979899";

char *ey_fmt_u64(char *end, uint64_t v) {
	while (v >= 100) {
		unsigned r = (unsigned)(v % 100);
		v /= 100;
		end -= 2;
		memcpy(end, digit_pairs + 2 * r, 2);
	}
	if (v >= 10) {
		end -= 2;
		memcpy(end, digit_pairs + 2 * v, 2);
	} else {
		*--end = (char)('0' + v);
	}
	return end;
}

char *ey_fmt_i64(char *end, int64_t v) {
	char *p = ey_fmt_u64(end, v < 0 ? (uint64_t)0 - (uint64_t)v : (uint64_t)v);
	if (v < 0) *--p = '-';
	return p;
}

void ey_buf_u64(ey_buf *b, uint64_t v, int width) {
	char s[24];
	char *p = ey_fmt_u64(s + sizeof s, v);
	while (s + sizeof s - p < width) *--p = '0';
	ey_buf_add(b, p, s + sizeof s - p);
}

void ey_buf_i64(ey_buf *b, int64_t v) {
	char s[24];
	char *p = ey_fmt_i64(s + sizeof s, v);
	ey_buf_add(b, p, s + sizeof s - p);
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
	if (!big) ey_out_of_memory();
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

_Noreturn void ey_expect_failed(const char *at, ey_v src, ey_v lsrc, const ey_type *lt, const void *l) {
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

void eyb_env_get(ey_v name, void *out, const ey_type *opt) {
	const char *v = getenv(EY_STR(name)->b);
	if (!v) {
		ey_none(opt, out);
		return;
	}
	ey_some_v(opt, out, ey_str_c(v));
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
	if (!items) ey_out_of_memory();
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

/* Objects that failing tests left: a fault jumps out of a test's code
 * without dropping what it holds (7.12 frees them). */
static uint64_t faulted;

/* The slots of the globals computed so far, and their types, which the
 * count drops. */
static struct {
	void *slot;
	const ey_type *ty;
} *globals;
static int nglobals, cglobals;

void ey_keep_global(void *slot, const ey_type *t) {
	if (nglobals == cglobals) {
		cglobals = cglobals ? cglobals * 2 : 16;
		globals = realloc(globals, sizeof *globals * (size_t)cglobals);
		if (!globals) ey_out_of_memory();
	}
	globals[nglobals].slot = slot;
	globals[nglobals].ty = t;
	nglobals++;
}

/* With EYE_ALLOCS set, a program writes how many objects it made and freed
 * when it exits, after dropping its globals, and how many of those not
 * freed failing tests left: "allocs 10 frees 8 faulted 2". */
static void report_allocs(void) {
	for (int i = 0; i < nglobals; i++) {
		ey_drop_at(globals[i].ty, globals[i].slot);
		memset(globals[i].slot, 0, globals[i].ty->size);
	}
	fflush(stdout);
	fprintf(stderr, "allocs %llu frees %llu faulted %llu\n", (unsigned long long)ey_allocs, (unsigned long long)ey_frees,
		(unsigned long long)faulted);
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
		uint64_t live = ey_allocs - ey_frees;
		volatile bool ok = true;
		ey_random_test(); /* each test draws the same numbers, as under eye test */
		if (setjmp(test_fault) == 0)
			t->run();
		else
			ok = false;
		int64_t took = now_ns() - start;
		total += took;
		ey_sandbox_end();
		if (!ok) faulted += ey_allocs - ey_frees - live;
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
	const char *count = getenv("EYE_ALLOCS");
	if (count && *count) atexit(report_allocs);
	if (!p->main) return run_tests(p, argc, argv);
	p->main();
	fflush(stdout);
	return 0;
}
