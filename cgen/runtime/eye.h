/* The core runtime of compiled Eyelang programs (ROADMAP 7.4).
 *
 * Every Eyelang value is one 64-bit word, ey_v:
 *
 *   - a scalar's bits: Bool, Char, Byte and the other fixed-width integers
 *     (signed ones sign-extended), Float (its IEEE bits), Duration (ns),
 *     Size (bytes), Date (days since 1970-01-01), Time (seconds since
 *     midnight) and () (0);
 *   - an Int: a small integer n as (n << 1) | 1, or a pointer to a big
 *     integer, which is never in the small range;
 *   - anything else: a pointer to a heap object, which starts with a
 *     reference count and its type's descriptor.
 *
 * A type with one variant of one field, named like the type (a newtype,
 * such as the prelude's Offset), is represented as its field.
 *
 * Counting is simple for now: each variable owns one reference, dropped
 * when it is overwritten and when its function returns; arguments are
 * borrowed and results owned. 7.5 makes it precise (Perceus). Counts are
 * not atomic: a program runs on one thread.
 */
#ifndef EYE_H
#define EYE_H

#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>
#include <string.h>

typedef uint64_t ey_v;

/* A growing run of bytes, for building strings. */
typedef struct ey_buf {
	char *b;
	int64_t len, cap;
} ey_buf;

/* ---------------------------------------------------------------- types */

enum {
	/* Scalars: no reference count. */
	EY_UNIT,
	EY_BOOL,
	EY_FLOAT,
	EY_CHAR,
	EY_BYTE,
	EY_INT32,
	EY_INT64,
	EY_UINT32,
	EY_UINT64,
	EY_DURATION,
	EY_SIZE,
	EY_DATE,
	EY_TIME,
	EY_NEVER,
	/* An Int: small, or a pointer to a big integer. */
	EY_INT,
	/* Heap objects. */
	EY_BIG,
	EY_STRING,
	EY_BYTES,
	EY_INSTANT,
	EY_LIST,
	EY_SET,
	EY_MAP,
	EY_TUPLE,
	EY_RECORD,
	EY_VARIANT,
	EY_FN,
	EY_DYN,
	EY_RANGE,
	EY_ITER,
	EY_LOOP /* a for loop's walk, which no Eyelang value is */
};

typedef struct ey_type ey_type;

typedef struct ey_variant {
	const char *name;
	uint32_t n;                  /* fields */
	const ey_type *const *fields;
	const char *const *names;    /* "" for a positional field */
} ey_variant;

/* A type's descriptor: what showing, comparing, hashing and freeing a
 * value of it needs. The back end writes one for each type a program
 * uses. */
struct ey_type {
	uint32_t kind;
	uint32_t n;                  /* args, fields or variants */
	const char *name;            /* a declared type's name */
	const ey_type *const *args;  /* elements, fields, captures; a dyn's or range's one */
	const char *const *names;    /* a record's field names */
	const ey_variant *variants;
};

extern const ey_type ey_t_unit, ey_t_bool, ey_t_float, ey_t_char, ey_t_byte, ey_t_int32,
	ey_t_int64, ey_t_uint32, ey_t_uint64, ey_t_duration, ey_t_size, ey_t_date, ey_t_time,
	ey_t_never, ey_t_int, ey_t_string, ey_t_bytes, ey_t_iter, ey_t_loop, ey_t_big, ey_t_instant;

static inline bool ey_counted(const ey_type *t) { return t->kind >= EY_INT; }

/* ---------------------------------------------------------------- objects */

#define EY_IMMORTAL INT64_MIN

typedef struct ey_obj {
	int64_t rc; /* EY_IMMORTAL for a static object */
	const ey_type *ty;
} ey_obj;

/* A record, a variant, a tuple or a range: fields in their declared order.
 * A range's are its start, its end and flags (EY_RANGE_*). */
typedef struct ey_agg {
	ey_obj h;
	uint32_t tag;
	uint32_t n;
	ey_v f[];
} ey_agg;

#define EY_RANGE_INCLUSIVE 1
#define EY_RANGE_NO_START 2
#define EY_RANGE_NO_END 4

/* A String (UTF-8) or Bytes; b ends with a NUL that len doesn't count.
 * cap is how many bytes b has room for, NUL included, once an append in
 * place has grown it; 0 means exactly len + 1. */
typedef struct ey_str {
	ey_obj h;
	int64_t len;
	int64_t cap;
	char b[];
} ey_str;

typedef struct ey_big {
	ey_obj h;
	int32_t neg;
	uint32_t n;   /* limbs, the highest not zero */
	uint32_t d[]; /* the magnitude, least significant limb first */
} ey_big;

typedef struct ey_list {
	ey_obj h;
	int64_t len, cap;
	ey_v *items;
} ey_list;

/* A Set (vals is NULL) or a Map, in insertion order; index holds each
 * key's position plus one, by hash, in a table of icap slots. */
typedef struct ey_map {
	ey_obj h;
	int64_t len, cap;
	ey_v *keys, *vals;
	int32_t *index;
	int64_t icap;
} ey_map;

typedef struct ey_fn ey_fn;
typedef ey_v (*ey_code)(ey_fn *self, const ey_v *args);

/* A closure: its code and what it captured. */
struct ey_fn {
	ey_obj h;
	ey_code code;
	uint32_t n;
	ey_v cap[];
};

typedef ey_v (*ey_method)(ey_v self, const ey_v *args);

/* A dyn Trait: the value and its type's table of the trait's methods. */
typedef struct ey_dyn {
	ey_obj h;
	const ey_method *methods;
	ey_v val;
} ey_dyn;

typedef struct ey_zone ey_zone;

typedef struct ey_instant {
	ey_obj h;
	int64_t ns; /* since 1970-01-01 00:00 UTC */
	const ey_zone *zone;
} ey_instant;

/* An Iter(a): the collection it walks and the lazy steps applied to it,
 * in order (SPECS.md §5). It is a value: each walk starts again from the
 * source and runs the steps' functions again. */
enum { EY_STEP_MAP, EY_STEP_FILTER, EY_STEP_TAKE, EY_STEP_DROP, EY_STEP_ENUMERATE, EY_STEP_ZIP, EY_STEP_FLATMAP };

typedef struct ey_step {
	int op;
	ey_v fn;            /* map, filter, flatMap */
	int64_t n;          /* take, drop */
	ey_v other;         /* zip: what the elements are paired with */
	const ey_type *out; /* the type of the elements the step gives */
} ey_step;

typedef struct ey_lazy {
	ey_obj h;
	ey_v src;            /* a List, Set, Map, Range, String or Bytes */
	const ey_type *elem; /* the type of src's elements */
	uint32_t n;
	ey_step s[];
} ey_lazy;

#define EY_OBJ(v) ((ey_obj *)(uintptr_t)(v))
#define EY_AGG(v) ((ey_agg *)(uintptr_t)(v))
#define EY_STR(v) ((ey_str *)(uintptr_t)(v))
#define EY_LIST(v) ((ey_list *)(uintptr_t)(v))
#define EY_MAP(v) ((ey_map *)(uintptr_t)(v))
#define EY_FNP(v) ((ey_fn *)(uintptr_t)(v))
#define EY_DYNP(v) ((ey_dyn *)(uintptr_t)(v))
#define EY_V(p) ((ey_v)(uintptr_t)(p))

void ey_free(ey_obj *o);
void *ey_alloc(size_t size, const ey_type *ty);

static inline void ey_dup(ey_v v) {
	if ((v & 1) == 0 && v != 0) {
		ey_obj *o = EY_OBJ(v);
		if (o->rc != EY_IMMORTAL) o->rc++;
	}
}

static inline void ey_drop(ey_v v) {
	if ((v & 1) == 0 && v != 0) {
		ey_obj *o = EY_OBJ(v);
		if (o->rc != EY_IMMORTAL && --o->rc == 0) ey_free(o);
	}
}

/* ---------------------------------------------------------------- faults */

/* Where the code is, for a fault: "file.eye:3:7". */
_Noreturn void ey_fault(const char *at, const char *msg);
_Noreturn void ey_faultf(const char *at, const char *fmt, ...);
_Noreturn void ey_unsupported(const char *at, const char *what);

/* ---------------------------------------------------------------- scalars */

static inline ey_v ey_f2v(double d) {
	ey_v v;
	memcpy(&v, &d, sizeof v);
	return v;
}
static inline double ey_v2f(ey_v v) {
	double d;
	memcpy(&d, &v, sizeof d);
	return d;
}

/* ---------------------------------------------------------------- Int */

#define EY_SMALL_MAX ((int64_t)0x3fffffffffffffff)
#define EY_SMALL_MIN (-EY_SMALL_MAX - 1)
#define EY_SMALL(n) ((((uint64_t)(int64_t)(n)) << 1) | 1)
#define EY_IS_SMALL(v) (((v)&1) != 0)
#define EY_SMALL_VAL(v) (((int64_t)(v)) >> 1)

ey_v ey_int_i64(int64_t n);
ey_v ey_int_u64(uint64_t n);
ey_v ey_int_big(int neg, const uint32_t *d, uint32_t n); /* normalized */
ey_v ey_int_add_slow(ey_v a, ey_v b);
ey_v ey_int_sub_slow(ey_v a, ey_v b);
ey_v ey_int_mul_slow(ey_v a, ey_v b);
ey_v ey_int_div(const char *at, ey_v a, ey_v b);
ey_v ey_int_mod(const char *at, ey_v a, ey_v b);
ey_v ey_int_neg(ey_v a);
int ey_int_cmp(ey_v a, ey_v b);
bool ey_int_eq(ey_v a, ey_v b);
double ey_int_to_f64(ey_v a);
bool ey_int_from_f64(double d, ey_v *out); /* truncates; false for NaN and infinities */
/* Whether a fits in [lo, hi], given as int64 bounds, or 0..UINT64_MAX
 * when unsigned64; and its two's complement bits. */
bool ey_int_fits(ey_v a, int64_t lo, int64_t hi, bool unsigned64);
uint64_t ey_int_bits(ey_v a);
bool ey_int_i64_of(ey_v a, int64_t *out);
void ey_int_write(ey_buf *b, ey_v a);
bool ey_int_parse(const char *s, int64_t len, int base, ey_v *out);

static inline ey_v ey_int_add(ey_v a, ey_v b) {
	int64_t r;
	if (EY_IS_SMALL(a) && EY_IS_SMALL(b) && !__builtin_add_overflow((int64_t)a, (int64_t)b - 1, &r))
		return (ey_v)r;
	return ey_int_add_slow(a, b);
}
static inline ey_v ey_int_sub(ey_v a, ey_v b) {
	int64_t r;
	if (EY_IS_SMALL(a) && EY_IS_SMALL(b) && !__builtin_sub_overflow((int64_t)a, (int64_t)b - 1, &r))
		return (ey_v)r;
	return ey_int_sub_slow(a, b);
}
static inline ey_v ey_int_mul(ey_v a, ey_v b) {
	int64_t r;
	if (EY_IS_SMALL(a) && EY_IS_SMALL(b) && !__builtin_mul_overflow(EY_SMALL_VAL(a), EY_SMALL_VAL(b), &r) &&
	    r >= EY_SMALL_MIN && r <= EY_SMALL_MAX)
		return EY_SMALL(r);
	return ey_int_mul_slow(a, b);
}
static inline bool ey_int_lt(ey_v a, ey_v b) {
	if (EY_IS_SMALL(a) && EY_IS_SMALL(b)) return (int64_t)a < (int64_t)b;
	return ey_int_cmp(a, b) < 0;
}

/* ---------------------------------------------------------------- buffers */

void ey_buf_add(ey_buf *b, const char *s, int64_t n);
void ey_buf_str(ey_buf *b, const char *s);
void ey_buf_char(ey_buf *b, char c);
void ey_buf_rune(ey_buf *b, uint32_t r);
void ey_buf_printf(ey_buf *b, const char *fmt, ...);
ey_v ey_buf_take(ey_buf *b); /* a String of what b holds; b is emptied */

/* ---------------------------------------------------------------- strings */

ey_v ey_str_new(const char *s, int64_t n, const ey_type *ty);
static inline ey_v ey_str_c(const char *s) { return ey_str_new(s, (int64_t)strlen(s), &ey_t_string); }
ey_v ey_concat(int n, const ey_v *parts);
bool ey_str_eq(ey_v a, ey_v b);
int ey_str_cmp(ey_v a, ey_v b);
int64_t ey_utf8_decode(const char *s, int64_t n, uint32_t *r); /* bytes taken; 1 with U+FFFD for an invalid byte */
bool ey_utf8_valid(const char *s, int64_t n);

/* ---------------------------------------------------------------- values in general */

bool ey_eq(const ey_type *t, ey_v a, ey_v b);
int ey_cmp(const ey_type *t, ey_v a, ey_v b);
uint64_t ey_hash(const ey_type *t, ey_v v);
void ey_show_to(ey_buf *b, const ey_type *t, ey_v v, bool nested);
ey_v ey_show(const ey_type *t, ey_v v);
void ey_float_write(ey_buf *b, double d);

/* ---------------------------------------------------------------- aggregates */

ey_agg *ey_agg_new(const ey_type *ty, uint32_t tag, uint32_t n);
/* Makes the aggregate *p points at uniquely owned, copying it if it is
 * shared, so that a field can be set in place. */
void ey_agg_own(ey_v *p);
ey_v ey_some(const ey_type *opt, ey_v v);  /* takes v */
ey_v ey_none(const ey_type *opt);
ey_v ey_ok(const ey_type *res, ey_v v);    /* takes v */
ey_v ey_err(const ey_type *res, ey_v v);   /* takes v */
ey_v ey_range(const ey_type *ty, ey_v start, ey_v end, uint32_t flags);

/* ---------------------------------------------------------------- closures */

ey_fn *ey_fn_new(const ey_type *ty, ey_code code, uint32_t n);
static inline ey_v ey_call(ey_v f, const ey_v *args) { return EY_FNP(f)->code(EY_FNP(f), args); }

/* ---------------------------------------------------------------- collections */

ey_v ey_list_new(const ey_type *ty, int64_t n, const ey_v *items); /* dups items */
ey_v ey_list_take(const ey_type *ty, int64_t n, ey_v *items);      /* takes items and the array */
ey_v ey_set_new(const ey_type *ty, int64_t n, const ey_v *items);
ey_v ey_map_new(const ey_type *ty, int64_t n, const ey_v *keys, const ey_v *vals);
int64_t ey_map_find(ey_map *m, ey_v key);
int64_t ey_coll_size(ey_v c);
/* Iter values (iter.c) */
ey_v ey_lazy_of(ey_v coll, const ey_type *elem); /* an Iter is its own */
ey_v ey_lazy_then(ey_v it, int op, ey_v fn, int64_t n, ey_v other, const ey_type *out);
ey_v ey_lazy_list(ey_v it, const ey_type *list); /* walks it to its end */
void ey_lazy_free(ey_obj *o);
/* A for loop's walk of anything a loop can walk, one element ahead. */
ey_v ey_loop_new(ey_v coll, const ey_type *elem);
bool ey_loop_more(ey_v loop);
ey_v ey_loop_next(ey_v loop);
void ey_loop_free(ey_obj *o);

/* ---------------------------------------------------------------- time */

struct ey_zone {
	const char *name;
	int32_t n;             /* transitions */
	const int64_t *at;     /* seconds since 1970 when each offset starts */
	const int32_t *offset; /* seconds east of UTC from then on */
	int32_t first;         /* the offset before the first transition */
};

void ey_zones(const ey_zone *const *zones, int n);
const ey_zone *ey_zone_find(const char *at, ey_v name);
ey_v ey_instant_new(int64_t ns, const ey_zone *z);
ey_v ey_instant_local(const char *at, int64_t days, int64_t secs, ey_v zone); /* a date and time in a zone */
ey_v ey_instant_next(const char *at, int64_t secs, ey_v zone); /* the next time the clock shows secs */
int64_t ey_days_from_civil(int64_t y, int64_t m, int64_t d);
void ey_civil_from_days(int64_t z, int64_t *y, int64_t *m, int64_t *d);

/* ---------------------------------------------------------------- the program */

typedef struct ey_test {
	const char *name;
	void (*run)(void);
} ey_test;

typedef struct ey_program {
	const ey_test *tests;
	int ntests;
	void (*main)(void); /* NULL without main */
	const ey_zone *const *zones;
	int nzones;
} ey_program;

int ey_start(const ey_program *p, int argc, char **argv);
ey_v ey_args(const ey_type *list);
void ey_print(int n, const ey_v *parts);
void ey_log(const char *level, int n, const ey_v *parts);
ey_v ey_now(void);

#endif
