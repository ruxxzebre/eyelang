/* The core runtime of compiled Eyelang programs (ROADMAP 7.4).
 *
 * Values are laid out by type (7.13, lir.Layouts; compilation.md section 2,
 * "Values laid out by type"). The back end holds each value as its type's
 * layout says, and tells the runtime how with the type's descriptor:
 *
 *   - a scalar at its own width: Bool and Byte (1 byte), Char, Int32,
 *     UInt32 and Time (4), Int64, UInt64, Float, Duration, Size and Date
 *     (8); () as one byte;
 *   - an enum, a variant type whose variants have no fields, as its
 *     variant's number (EY_R_ENUM);
 *   - a word, ey_v (EY_R_WORD): an Int (a small integer n as (n << 1) | 1,
 *     or a pointer to a big integer), a pointer to an object of the runtime
 *     (a String or Bytes, a List, Set or Map, a closure, a dyn value, an
 *     Iter, an Instant, a Range), a box (a pointer to an object holding a
 *     large or recursive record, tuple or variant, or for a variant without
 *     fields the immediate (n << 1) | 1), or a variant type whose variants
 *     without fields are spare values of such a word: Option(Int)'s None is
 *     0, its Some(n) is n's word;
 *   - a struct (EY_R_STRUCT): a record, tuple or variant type of at most
 *     64 bytes, inline, at the offsets its descriptor gives.
 *
 * A word below 4096 or odd is never a pointer: that is what a variant type
 * uses for the variants without fields it holds in a word, and what makes
 * ey_dup and ey_drop of any word safe. All-zero bits are a value whose drop
 * does nothing.
 *
 * A type with one variant of one field, named like the type (a newtype,
 * such as the prelude's Offset), is represented as its field.
 *
 * The low IR says where each value is dup'ed and dropped (lir.Count, 7.5):
 * a value is dropped after its last use, a parameter a function only reads
 * is borrowed, and an update whose operand dies there takes it (the eyo_
 * functions) and changes it in place when it holds the only reference.
 * Built-ins borrow their arguments and give owned results; one that makes a
 * value that isn't a word or a scalar writes it where its out argument
 * points. Counts are not atomic: a program runs on one thread.
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
	/* The runtime's objects, held as words. */
	EY_BIG,
	EY_STRING,
	EY_BYTES,
	EY_INSTANT,
	EY_LIST,
	EY_SET,
	EY_MAP,
	EY_FN,
	EY_DYN,
	EY_RANGE,
	EY_ITER,
	EY_LOOP, /* a for loop's walk, which no Eyelang value is */
	/* Declared and structural types, laid out by type. */
	EY_TUPLE,
	EY_RECORD,
	EY_VARIANT
};

/* How a value of a type is held (lir.Repr). */
enum { EY_R_NONE, EY_R_SCALAR, EY_R_WORD, EY_R_ENUM, EY_R_STRUCT };

/* How a variant type's value says which variant it is (lir.TagKind). */
enum { EY_TAG_NONE, EY_TAG_FIELD, EY_TAG_NICHE, EY_TAG_BOX };

typedef struct ey_type ey_type;

typedef struct ey_variant {
	const char *name;
	uint32_t n;                   /* fields */
	const ey_type *const *fields;
	const char *const *names;     /* "" for a positional field */
	const uint32_t *offs;         /* where each field is: in the value, or in a box's object */
	uint64_t value;               /* its number, its niche's value, or its immediate word */
	uint32_t objsize;             /* in a box: the size of its object */
} ey_variant;

/* A type's descriptor: its layout, and what showing, comparing, hashing,
 * copying and freeing a value of it needs. The back end writes one for each
 * type a program uses. */
struct ey_type {
	uint32_t kind;
	uint32_t n;                   /* args, fields or variants */
	uint32_t size, align;         /* of a value where it is held */
	uint8_t repr;                 /* EY_R_* */
	uint8_t boxed;                /* a record, tuple or variant type in a box */
	uint8_t tag;                  /* a variant type's EY_TAG_* */
	uint8_t tagsize;              /* EY_TAG_FIELD, EY_TAG_NICHE: the tag's bytes, at tagoff */
	uint32_t tagoff;
	uint32_t objsize;             /* a boxed record or tuple, a closure, a dyn value: its object's size */
	const char *name;             /* a declared type's name */
	const ey_type *const *args;   /* elements, fields, captures; a dyn's, a range's one */
	const char *const *names;     /* a record's field names */
	const uint32_t *offs;         /* where each field or capture is */
	const ey_variant *variants;
	void (*drop)(void *);         /* a counted struct's: drops what it holds */
	void (*dup)(void *);          /* a counted struct's: dups what it holds */
};

extern const ey_type ey_t_unit, ey_t_bool, ey_t_float, ey_t_char, ey_t_byte, ey_t_int32,
	ey_t_int64, ey_t_uint32, ey_t_uint64, ey_t_duration, ey_t_size, ey_t_date, ey_t_time,
	ey_t_never, ey_t_int, ey_t_string, ey_t_bytes, ey_t_iter, ey_t_loop, ey_t_big, ey_t_instant;

static inline bool ey_counted(const ey_type *t) {
	return t->repr == EY_R_WORD || (t->repr == EY_R_STRUCT && t->drop != NULL);
}

/* ---------------------------------------------------------------- objects */

#define EY_IMMORTAL INT32_MIN

/* No value is larger than this (lir.MaxInline): a larger record is boxed.
 * The runtime holds a value it works on in a buffer of this size. */
#define EY_MAXVAL 64
typedef struct ey_tmp {
	_Alignas(8) char b[EY_MAXVAL];
} ey_tmp;

/* Every heap object starts with this: its count (negative for a static
 * object, which is never freed; a count that would overflow becomes one),
 * 32 bits its kind uses (a boxed variant's number), and its descriptor. */
typedef struct ey_obj {
	int32_t rc;
	uint32_t tag;
	const ey_type *ty;
} ey_obj;

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

/* A List: its elements laid out one after another, each its element
 * type's size. */
typedef struct ey_list {
	ey_obj h;
	int64_t len, cap;
	char *items;
} ey_list;

/* A Set (vals is NULL) or a Map, in insertion order; index holds each
 * key's hash and position, by hash, in a table of icap slots (coll.c).
 * last is the position the last lookup found, or -1. */
typedef struct ey_map {
	ey_obj h;
	int64_t len, cap;
	char *keys, *vals;
	uint64_t *index;
	int64_t icap;
	int64_t last;
} ey_map;

/* A range: its start and end as words (an Int's word, or a scalar's bits)
 * and flags (EY_RANGE_*). */
typedef struct ey_range_obj {
	ey_obj h;
	ey_v start, end;
	uint32_t flags;
} ey_range_obj;

#define EY_RANGE_INCLUSIVE 1
#define EY_RANGE_NO_START 2
#define EY_RANGE_NO_END 4

typedef struct ey_fn ey_fn;
/* A closure's code: it writes its result where out points, and reads its
 * arguments, which it borrows, where args[i] point. */
typedef void (*ey_code)(ey_fn *self, void *out, void *const *args);

/* A closure: its code, and what it captured after it, as the closure's
 * descriptor lays them out. */
struct ey_fn {
	ey_obj h;
	ey_code code;
	uint64_t pad;
};

/* A dyn method: the value it is called on, where its result goes, and its
 * arguments. */
typedef void (*ey_method)(void *self, void *out, void *const *args);

/* A dyn Trait: its type's table of the trait's methods, and the value
 * after it, as its descriptor's args[0] lays it out. */
typedef struct ey_dyn {
	ey_obj h;
	const ey_method *methods;
} ey_dyn;
#define EY_DYN_VAL(v) ((void *)((char *)(uintptr_t)(v) + sizeof(ey_dyn)))

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
#define EY_STR(v) ((ey_str *)(uintptr_t)(v))
#define EY_LIST(v) ((ey_list *)(uintptr_t)(v))
#define EY_MAP(v) ((ey_map *)(uintptr_t)(v))
#define EY_FNP(v) ((ey_fn *)(uintptr_t)(v))
#define EY_DYNP(v) ((ey_dyn *)(uintptr_t)(v))
#define EY_RANGEP(v) ((ey_range_obj *)(uintptr_t)(v))
#define EY_V(p) ((ey_v)(uintptr_t)(p))
/* the value of type T off bytes into what p points at */
#define EY_AT(T, p, off) (*(T *)((char *)(p) + (off)))

/* Blocks of memory from the runtime's allocator: free lists by size class
 * for small blocks, malloc for large ones. Freeing or growing a block
 * needs the size it was allocated with. */
void *ey_mem_alloc(size_t n);
void ey_mem_free(void *p, size_t n);
void *ey_mem_realloc(void *p, size_t old, size_t n);

/* Objects: ey_alloc makes one with a count of 1, which the caller fills;
 * ey_free frees one whose count reached 0, and what it holds; ey_free_obj
 * gives back its memory. ey_allocs and ey_frees count objects made and
 * freed (EYE_ALLOCS). */
void ey_free(ey_obj *o);
void ey_free_obj(ey_obj *o, size_t size);
void *ey_alloc(size_t size, const ey_type *ty);
extern uint64_t ey_allocs, ey_frees;

/* whether a word points at an object */
static inline bool ey_is_obj(ey_v v) { return (v & 1) == 0 && v >= 4096; }

static inline void ey_dup(ey_v v) {
	if (ey_is_obj(v)) {
		ey_obj *o = EY_OBJ(v);
		if (o->rc > 0) o->rc++; /* past INT32_MAX it wraps to static */
	}
}

static inline void ey_drop(ey_v v) {
	if (ey_is_obj(v)) {
		ey_obj *o = EY_OBJ(v);
		if (o->rc > 0 && --o->rc == 0) ey_free(o);
	}
}

/* dup or drop the value of type t at p */
static inline void ey_dup_at(const ey_type *t, void *p) {
	if (t->repr == EY_R_WORD)
		ey_dup(*(ey_v *)p);
	else if (t->repr == EY_R_STRUCT && t->dup)
		t->dup(p);
}
static inline void ey_drop_at(const ey_type *t, void *p) {
	if (t->repr == EY_R_WORD)
		ey_drop(*(ey_v *)p);
	else if (t->repr == EY_R_STRUCT && t->drop)
		t->drop(p);
}
/* dup n values of type t laid out from p */
void ey_dup_n(const ey_type *t, void *p, int64_t n);
void ey_drop_n(const ey_type *t, void *p, int64_t n);

_Noreturn void ey_out_of_memory(void);

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

/* A scalar's, an enum's or a word's value at p as a word, and back: signed
 * scalars are sign-extended, Float is its bits. What the runtime works on
 * as words. */
ey_v ey_load_v(const ey_type *t, const void *p);
void ey_store_v(const ey_type *t, void *p, ey_v v);

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
ey_v ey_int_div_slow(const char *at, ey_v a, ey_v b);
ey_v ey_int_mod_slow(const char *at, ey_v a, ey_v b);
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
/* Int's / is floor division, and % its remainder, which has the divisor's
 * sign: -7 / 2 is -4 and -7 % 2 is 1. Small values divide here; a big one,
 * a zero divisor (a fault) and SMALL_MIN / -1 (a big result) go to the
 * slow path. */
static inline ey_v ey_int_div(const char *at, ey_v a, ey_v b) {
	if (EY_IS_SMALL(a) && EY_IS_SMALL(b) && b != EY_SMALL(0) && b != EY_SMALL(-1)) {
		int64_t x = EY_SMALL_VAL(a), y = EY_SMALL_VAL(b), q = x / y, r = x % y;
		if (r != 0 && ((r < 0) != (y < 0))) q--;
		return EY_SMALL(q);
	}
	return ey_int_div_slow(at, a, b);
}
static inline ey_v ey_int_mod(const char *at, ey_v a, ey_v b) {
	if (EY_IS_SMALL(a) && EY_IS_SMALL(b) && b != EY_SMALL(0) && b != EY_SMALL(-1)) {
		int64_t x = EY_SMALL_VAL(a), y = EY_SMALL_VAL(b), r = x % y;
		if (r != 0 && ((r < 0) != (y < 0))) r += y;
		return EY_SMALL(r);
	}
	return ey_int_mod_slow(at, a, b);
}
/* / and % by a constant k, small and neither 0 nor -1, which the back end
 * writes for a literal divisor: the C compiler turns the division into a
 * multiplication. */
static inline ey_v ey_int_div_k(const char *at, ey_v a, int64_t k) {
	if (EY_IS_SMALL(a)) {
		int64_t x = EY_SMALL_VAL(a), q = x / k, r = x % k;
		if (r != 0 && ((r < 0) != (k < 0))) q--;
		return EY_SMALL(q);
	}
	return ey_int_div_slow(at, a, EY_SMALL(k));
}
static inline ey_v ey_int_mod_k(const char *at, ey_v a, int64_t k) {
	if (EY_IS_SMALL(a)) {
		int64_t x = EY_SMALL_VAL(a), r = x % k;
		if (r != 0 && ((r < 0) != (k < 0))) r += k;
		return EY_SMALL(r);
	}
	return ey_int_mod_slow(at, a, EY_SMALL(k));
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
/* Integers in decimal: ey_fmt_* write v's digits so that they end at end,
 * and give where they start; ey_buf_u64 pads with zeros to width. */
char *ey_fmt_u64(char *end, uint64_t v);
char *ey_fmt_i64(char *end, int64_t v);
void ey_buf_u64(ey_buf *b, uint64_t v, int width);
void ey_buf_i64(ey_buf *b, int64_t v);
ey_v ey_buf_take(ey_buf *b); /* a String of what b holds; b is emptied */

/* ---------------------------------------------------------------- strings */

ey_v ey_str_new(const char *s, int64_t n, const ey_type *ty);
static inline ey_v ey_str_c(const char *s) { return ey_str_new(s, (int64_t)strlen(s), &ey_t_string); }
ey_v ey_concat(int n, const ey_v *parts);
bool ey_str_eq(ey_v a, ey_v b);
int ey_str_cmp(ey_v a, ey_v b);
uint64_t ey_str_hash(ey_v s);
int64_t ey_utf8_decode(const char *s, int64_t n, uint32_t *r); /* bytes taken; 1 with U+FFFD for an invalid byte */
bool ey_utf8_valid(const char *s, int64_t n);

/* ---------------------------------------------------------------- values in general */

/* By the type's descriptor, of values where a and b point. */
bool ey_eq(const ey_type *t, const void *a, const void *b);
int ey_cmp(const ey_type *t, const void *a, const void *b);
uint64_t ey_hash(const ey_type *t, const void *p);
void ey_show_to(ey_buf *b, const ey_type *t, const void *p, bool nested);
ey_v ey_show(const ey_type *t, const void *p);
ey_v ey_show_int(ey_v v); /* an Int in decimal */
void ey_float_write(ey_buf *b, double d);

/* ---------------------------------------------------------------- records, tuples and variants */

/* Which variant the value of variant type t at p is, and where field i of
 * it is. */
uint32_t ey_variant_of(const ey_type *t, const void *p);
void *ey_variant_field(const ey_type *t, const void *p, uint32_t tag, uint32_t i);
/* Writes the variant tag of t with fields[i] where out points, taking the
 * fields; a box is allocated for a boxed type. */
void ey_variant_make(const ey_type *t, void *out, uint32_t tag, const void *const *fields);
/* Where field i of the record or tuple of type t at p is, and a new one,
 * taking its fields. */
void *ey_field(const ey_type *t, const void *p, uint32_t i);
void ey_tuple_make(const ey_type *t, void *out, const void *const *fields);
/* Makes the box *p points at uniquely owned, copying it if it is shared,
 * so that a field can be set in place. */
void ey_box_own(ey_v *p);
/* Option and Result, taking v: v points at a value of the payload's type;
 * the _v forms take a scalar's, an enum's or a word's value as a word. */
static inline bool ey_niche_word(const ey_type *t) { return t->tag == EY_TAG_NICHE && t->tagsize == 8 && t->size == 8; }
/* An Option whose niche fills a word (Option(Int), Option(String),
 * Option(List(a))) is its payload's word, or None's. */
static inline void ey_some(const ey_type *opt, void *out, const void *v) {
	if (ey_niche_word(opt))
		memcpy(out, v, 8);
	else
		ey_variant_make(opt, out, 0, (const void *const[]){v});
}
static inline void ey_none(const ey_type *opt, void *out) {
	if (ey_niche_word(opt))
		memcpy(out, &opt->variants[1].value, 8);
	else
		ey_variant_make(opt, out, 1, NULL);
}
void ey_ok(const ey_type *res, void *out, const void *v);
void ey_err(const ey_type *res, void *out, const void *v);
void ey_some_v(const ey_type *opt, void *out, ey_v v);
void ey_ok_v(const ey_type *res, void *out, ey_v v);
void ey_err_v(const ey_type *res, void *out, ey_v v);
ey_v ey_range(const ey_type *ty, ey_v start, ey_v end, uint32_t flags);

/* ---------------------------------------------------------------- closures */

ey_fn *ey_fn_new(const ey_type *ty, ey_code code);
static inline void ey_call(ey_v f, void *out, void *const *args) { EY_FNP(f)->code(EY_FNP(f), out, args); }
/* a capture of a closure */
#define EY_CAP(T, f, off) EY_AT(T, f, off)

/* ---------------------------------------------------------------- collections */

/* A List of n elements of its element type from items: _new dups them,
 * _from takes them (a literal's list), _take takes them and the array. */
ey_v ey_list_new(const ey_type *ty, int64_t n, const void *items);
ey_v ey_list_from(const ey_type *ty, int64_t n, void *const *items);
ey_v ey_list_take(const ey_type *ty, int64_t n, void *items);
/* the address of element i of a list */
static inline void *ey_list_item(ey_list *l, int64_t i) { return l->items + i * (int64_t)l->h.ty->args[0]->size; }
ey_v ey_set_new(const ey_type *ty, int64_t n, const void *items);
ey_v ey_set_from(const ey_type *ty, int64_t n, void *const *items);
ey_v ey_map_new(const ey_type *ty, int64_t n, const void *keys, const void *vals);
ey_v ey_map_from(const ey_type *ty, int64_t n, void *const *keys, void *const *vals);
int64_t ey_map_find(ey_map *m, const void *key);
int64_t ey_coll_size(ey_v c);
/* Iter values (iter.c) */
ey_v ey_lazy_of(ey_v coll, const ey_type *elem); /* an Iter is its own */
ey_v ey_lazy_then(ey_v it, int op, ey_v fn, int64_t n, ey_v other, const ey_type *out);
ey_v ey_lazy_list(ey_v it, const ey_type *list); /* walks it to its end */
void ey_lazy_free(ey_obj *o);
/* A for loop's walk of anything a loop can walk, one element ahead; next
 * writes the element, which the caller owns, where out points. */
ey_v ey_loop_new(ey_v coll, const ey_type *elem);
bool ey_loop_more(ey_v loop);
void ey_loop_next(ey_v loop, void *out);
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
/* a global's value, of type t, which the count of objects drops at exit */
void ey_keep_global(void *slot, const ey_type *t);
ey_v ey_args(const ey_type *list);
void ey_print(int n, const ey_v *parts);
void ey_log(const char *level, int n, const ey_v *parts);
ey_v ey_now(void);

#endif
