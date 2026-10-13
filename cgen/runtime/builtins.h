/* The built-in functions compiled code calls, by the types of their
 * arguments. Arguments are borrowed and results owned. An element of a
 * collection is passed by its address; a function that makes an Option, a
 * Result or an element writes it where its out argument points, and one
 * that makes an Option, a Result or a collection takes its type's
 * descriptor last; one that can fault takes where the call is first. */
#ifndef EYE_BUILTINS_H
#define EYE_BUILTINS_H

#include "eye.h"
#include "data.h"

/* strings */
ey_v eyb_str_size(ey_v s);
ey_v eyb_str_take(const char *at, ey_v s, ey_v n);
ey_v eyb_str_drop(const char *at, ey_v s, ey_v n);
void eyb_str_index_of(ey_v s, ey_v part, ey_v from, bool has_from, void *out, const ey_type *opt);
void eyb_str_slice(ey_v s, ey_v range, void *out, const ey_type *opt);
void eyb_str_step(const char *at, ey_v s, ey_v off, ey_v by, bool has_by, bool forward, void *out, const ey_type *opt);
void eyb_str_char_at(ey_v s, ey_v off, void *out, const ey_type *opt);
ey_v eyb_str_replace(ey_v s, ey_v a, ey_v b);
ey_v eyb_str_words(ey_v s, const ey_type *list);
ey_v eyb_str_trim(ey_v s);
bool eyb_str_starts_with(ey_v s, ey_v x);
bool eyb_str_ends_with(ey_v s, ey_v x);
bool eyb_str_contains(ey_v s, ey_v x);
ey_v eyb_str_split(ey_v s, ey_v sep, const ey_type *list);
ey_v eyb_str_lines(ey_v s, const ey_type *list);
ey_v eyb_str_chars(ey_v s, const ey_type *list);
ey_v eyb_str_case(ey_v s, bool upper);
ey_v eyb_str_bytes(ey_v s);
void eyb_from_code(ey_v n, void *out, const ey_type *opt);
void eyb_parse_int(const char *at, ey_v s, ey_v base, bool has_base, void *out, const ey_type *res);
void eyb_parse_float(ey_v s, void *out, const ey_type *res);
void eyb_float_to_int(double d, void *out, const ey_type *opt);

/* Bytes */
void eyb_bytes_index(ey_v b, ey_v i, void *out, const ey_type *opt);
uint8_t eyb_bytes_at(const char *at, ey_v b, ey_v i);
ey_v eyb_bytes_slice(ey_v b, ey_v start, ey_v end, uint32_t flags);
bool eyb_bytes_contains(ey_v b, uint8_t x);
void eyb_bytes_to_text(ey_v b, void *out, const ey_type *opt);
void eyb_bounds(ey_v start, ey_v end, uint32_t flags, int64_t n, int64_t *lo, int64_t *hi);

/* collections */
ey_v eyb_list_add(ey_v l, const void *x);
ey_v eyb_list_concat(ey_v a, ey_v b);
ey_v eyb_set_add(ey_v s, const void *x);
ey_v eyb_set_union(ey_v a, ey_v b);
ey_v eyb_map_insert(ey_v m, const void *k, const void *v);
ey_v eyb_map_remove(ey_v m, const void *k);
bool eyb_map_contains(ey_v m, const void *k);
bool eyb_list_contains(ey_v l, const void *x);
/* xs[i] and m[k] are inline: the element's type is the Option's payload,
 * which the compiler knows from the descriptor, so the dup is the type's. */
static inline void eyb_list_index(ey_v l, ey_v i, void *out, const ey_type *opt) {
	ey_list *a = EY_LIST(l);
	int64_t k = EY_IS_SMALL(i) ? EY_SMALL_VAL(i) : -1;
	if (k < 0 || k >= a->len) {
		ey_none(opt, out);
		return;
	}
	const ey_type *et = opt->variants[0].fields[0];
	void *p = a->items + k * (int64_t)et->size;
	ey_dup_at(et, p);
	ey_some(opt, out, p);
}
static inline void eyb_map_get(ey_v mv, const void *k, void *out, const ey_type *opt) {
	ey_map *m = EY_MAP(mv);
	int64_t i = ey_map_find(m, k);
	if (i < 0) {
		ey_none(opt, out);
		return;
	}
	const ey_type *vt = opt->variants[0].fields[0];
	void *p = m->vals + i * (int64_t)vt->size;
	ey_dup_at(vt, p);
	ey_some(opt, out, p);
}
void eyb_list_at(const char *at, ey_v l, ey_v i, void *out);
ey_v eyb_list_slice(ey_v l, ey_v start, ey_v end, uint32_t flags);
void eyb_seq_end(ey_v c, bool last, void *out, const ey_type *opt);
ey_v eyb_seq_map(ey_v c, ey_v f, const ey_type *rt);
ey_v eyb_seq_filter(ey_v c, ey_v f, const ey_type *rt);
void eyb_seq_find(ey_v c, ey_v f, void *out, const ey_type *opt);
ey_v eyb_seq_test(ey_v c, ey_v f, int how); /* a Bool, or for count an Int */
void eyb_seq_reduce(ey_v c, const void *init, ey_v f, void *out, const ey_type *acct);
ey_v eyb_seq_sum(ey_v c); /* an Int, or a Float's bits */
ey_v eyb_seq_enumerate(ey_v c, const ey_type *rt);
ey_v eyb_seq_take_drop(const char *at, ey_v c, ey_v count, bool take, const ey_type *rt);
ey_v eyb_seq_reverse(ey_v c, const ey_type *rt);
ey_v eyb_seq_sort(ey_v c, const ey_type *rt);
ey_v eyb_map_list(ey_v m, int which, const ey_type *rt);
ey_v eyb_seq_convert(ey_v c, const ey_type *rt);
ey_v eyb_seq_join(ey_v c, ey_v sep);
ey_v eyb_seq_flat_map(ey_v c, ey_v f, const ey_type *rt);
ey_v eyb_seq_zip(ey_v a, ey_v b, const ey_type *rt);
ey_v eyb_range_list(ey_v r, const ey_type *rt);
void eyb_seq_average(ey_v c, void *out, const ey_type *opt);

/* Iters: what stops as soon as it knows (iter.c) */
void eyb_iter_first(ey_v it, void *out, const ey_type *opt);
void eyb_iter_find(ey_v it, ey_v f, void *out, const ey_type *opt);
bool eyb_iter_test(ey_v it, ey_v f, int how);
bool eyb_iter_contains(ey_v it, const void *x);
bool eyb_iter_is_empty(ey_v it);

/* Int */
void eyb_int_div_rem(ey_v a, ey_v b, bool rem, void *out, const ey_type *opt);

/* Updates whose operand's last use they are (lir.Count's inplace): each
 * takes its first argument, changes it in place when that is its only
 * reference, and copies it first otherwise, so a loop that grows a value
 * costs amortised O(1) a step instead of a copy a step. */
ey_v eyo_str_append(ey_v s, ey_v t);
ey_v eyo_concat(ey_v s, int n, const ey_v *parts);
ey_v eyo_list_concat(ey_v l, ey_v other);
ey_v eyo_list_add(ey_v l, const void *x);
ey_v eyo_set_add(ey_v s, const void *x);
ey_v eyo_map_insert(ey_v m, const void *k, const void *v);
ey_v eyo_set_union(ey_v s, ey_v other);
ey_v eyo_map_remove(ey_v m, const void *k);
/* The slot of element i of the list *p points at, for an assignment to
 * it, `xs[i] = v` (SPECS.md §7): *p is made unique first, copied if it is
 * shared, and an index out of range is a fault. */
void *eyo_list_slot(const char *at, ey_v *p, ey_v i);

/* Bytes of a List(Byte): toBytes(list) */
ey_v eyb_list_to_bytes(ey_v l);

/* fixed-width integers, as words (ey_load_v) */
enum { EYF_WADD, EYF_WSUB, EYF_WMUL, EYF_AND, EYF_OR, EYF_XOR, EYF_NOT, EYF_SHL, EYF_SHR };
ey_v eyb_fixed(const char *at, int op, int bits, bool sign, ey_v a, ey_v b);
void eyb_fixed_checked(int op, int bits, bool sign, ey_v a, ey_v b, void *out, const ey_type *opt);
void eyb_fixed_from(ey_v n, int bits, bool sign, void *out, const ey_type *opt);
ey_v eyb_fixed_to_int(int bits, bool sign, ey_v a);

/* enums */
ey_v eyb_enum_variants(const ey_type *t, const ey_type *list);
void eyb_enum_parse(ey_v name, void *out, const ey_type *opt);

/* faults and output */
_Noreturn void ey_unreachable(const char *at, ey_v why);
_Noreturn void ey_expect_failed(const char *at, ey_v src, ey_v lsrc, const ey_type *lt, const void *l);
void ey_eprintln(ey_v s);
void eyb_env_get(ey_v name, void *out, const ey_type *opt);

static inline int64_t ey_small_or_fault(const char *at, ey_v n) {
	if (!EY_IS_SMALL(n)) ey_fault(at, "the number is too large here");
	return EY_SMALL_VAL(n);
}

/* a count of elements: an Int that isn't negative */
static inline int64_t ey_count(const char *at, ey_v n) {
	int64_t k;
	if (!ey_int_i64_of(n, &k) || k < 0) ey_fault(at, "a count is an Int that isn't negative");
	return k;
}

/* time */
ey_v eyb_instant_add(ey_v i, int64_t d);
int64_t eyb_instant_sub(ey_v a, ey_v b);
ey_v eyb_instant_unix(ey_v i);
int64_t eyb_instant_date(const char *at, ey_v i, ey_v zone);
int32_t eyb_instant_time(const char *at, ey_v i, ey_v zone);
ey_v eyb_time_next(const char *at, int32_t secs, ey_v zone, ey_v after);
ey_v eyb_date_at(const char *at, int64_t day, int32_t secs, ey_v zone);
ey_v eyb_date_format(int64_t day, ey_v pattern);
ey_v eyb_time_format(int32_t secs, ey_v pattern);
bool eyb_time_range_contains(const char *at, ey_v range, int32_t t);

/* fs and process */
void ey_sandbox(bool on);
void ey_sandbox_end(void);
void eyb_fs_read_text(ey_v p, void *out, const ey_type *res);
void eyb_fs_read_bytes(ey_v p, void *out, const ey_type *res);
void eyb_fs_lines(ey_v p, void *out, const ey_type *res);
/* writes a String's or Bytes' bytes */
void eyb_fs_write_text(ey_v p, ey_v text, bool append, void *out, const ey_type *res);
bool eyb_fs_exists(ey_v p);
void eyb_fs_list(ey_v p, void *out, const ey_type *res);
void eyb_fs_make_dir(ey_v p, void *out, const ey_type *res);
void eyb_fs_remove(ey_v p, void *out, const ey_type *res);
/* random (random.c): a test starts from a fixed seed, ey_random_test */
void ey_random_seed(uint64_t hi, uint64_t lo);
void ey_random_test(void);
void eyb_random_int(const char *at, ey_v range, void *out, const ey_type *opt);
double eyb_random_float(void);
void eyb_random_pick(ey_v xs, void *out, const ey_type *opt);
ey_v eyb_random_shuffle(ey_v xs);

void eyb_process_run(ey_v program, ey_v args, ey_v input, const void *dir, const ey_type *dirt, void *out, const ey_type *res);

#endif
