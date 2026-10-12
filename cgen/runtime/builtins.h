/* The built-in functions compiled code calls, by the types of their
 * arguments. Arguments are borrowed and results owned; a function that
 * makes an Option, a Result or a collection takes its type's descriptor
 * last; one that can fault takes where the call is first. */
#ifndef EYE_BUILTINS_H
#define EYE_BUILTINS_H

#include "eye.h"

/* strings */
ey_v eyb_str_size(ey_v s);
ey_v eyb_str_take(const char *at, ey_v s, ey_v n);
ey_v eyb_str_drop(const char *at, ey_v s, ey_v n);
ey_v eyb_str_index_of(ey_v s, ey_v part, ey_v from, bool has_from, const ey_type *opt);
ey_v eyb_str_slice(ey_v s, ey_v range, const ey_type *opt);
ey_v eyb_str_step(const char *at, ey_v s, ey_v off, ey_v by, bool has_by, bool forward, const ey_type *opt);
ey_v eyb_str_char_at(ey_v s, ey_v off, const ey_type *opt);
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
ey_v eyb_from_code(ey_v n, const ey_type *opt);
ey_v eyb_parse_int(const char *at, ey_v s, ey_v base, bool has_base, const ey_type *res);
ey_v eyb_parse_float(ey_v s, const ey_type *res);
ey_v eyb_float_to_int(double d, const ey_type *opt);

/* Bytes */
ey_v eyb_bytes_index(ey_v b, ey_v i, const ey_type *opt);
ey_v eyb_bytes_at(const char *at, ey_v b, ey_v i);
ey_v eyb_bytes_slice(ey_v b, ey_v start, ey_v end, uint32_t flags);
bool eyb_bytes_contains(ey_v b, ey_v x);
ey_v eyb_bytes_to_text(ey_v b, const ey_type *opt);
void eyb_bounds(ey_v start, ey_v end, uint32_t flags, int64_t n, int64_t *lo, int64_t *hi);

/* collections */
ey_v eyb_list_add(ey_v l, ey_v x);
ey_v eyb_list_concat(ey_v a, ey_v b);
ey_v eyb_set_add(ey_v s, ey_v x);
ey_v eyb_set_union(ey_v a, ey_v b);
ey_v eyb_map_insert(ey_v m, ey_v k, ey_v v);
ey_v eyb_map_remove(ey_v m, ey_v k);
bool eyb_map_contains(ey_v m, ey_v k);
bool eyb_list_contains(ey_v l, ey_v x);
ey_v eyb_map_get(ey_v m, ey_v k, const ey_type *opt);
ey_v eyb_list_index(ey_v l, ey_v i, const ey_type *opt);
ey_v eyb_list_at(const char *at, ey_v l, ey_v i);
ey_v eyb_list_slice(ey_v l, ey_v start, ey_v end, uint32_t flags);
ey_v eyb_seq_end(ey_v c, bool last, const ey_type *opt);
ey_v eyb_seq_map(ey_v c, ey_v f, const ey_type *rt);
ey_v eyb_seq_filter(ey_v c, ey_v f, const ey_type *rt);
ey_v eyb_seq_find(ey_v c, ey_v f, const ey_type *opt);
ey_v eyb_seq_test(ey_v c, ey_v f, int how);
ey_v eyb_seq_reduce(ey_v c, ey_v init, ey_v f, const ey_type *acct);
ey_v eyb_seq_sum(ey_v c);
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
ey_v eyb_seq_average(ey_v c, const ey_type *opt);

/* Iters: what stops as soon as it knows (iter.c) */
ey_v eyb_iter_first(ey_v it, const ey_type *opt);
ey_v eyb_iter_find(ey_v it, ey_v f, const ey_type *opt);
ey_v eyb_iter_test(ey_v it, ey_v f, int how);
bool eyb_iter_contains(ey_v it, ey_v x);
bool eyb_iter_is_empty(ey_v it);

/* Int */
ey_v eyb_int_div_rem(ey_v a, ey_v b, bool rem, const ey_type *opt);

/* Updates of a variable by itself, `s += t` or `m = m.insert(k, v)`: each
 * takes the variable's value, changes it in place when that is its only
 * reference, and copies it first otherwise, so a loop that grows a value
 * costs amortised O(1) a step instead of a copy a step. */
ey_v eyo_str_append(ey_v s, ey_v t);
ey_v eyo_concat(ey_v s, int n, const ey_v *parts);
ey_v eyo_list_concat(ey_v l, ey_v other);
ey_v eyo_list_add(ey_v l, ey_v x);
ey_v eyo_set_add(ey_v s, ey_v x);
ey_v eyo_map_insert(ey_v m, ey_v k, ey_v v);

/* fixed-width integers */
enum { EYF_WADD, EYF_WSUB, EYF_WMUL, EYF_AND, EYF_OR, EYF_XOR, EYF_NOT, EYF_SHL, EYF_SHR };
ey_v eyb_fixed(const char *at, int op, int bits, bool sign, ey_v a, ey_v b);
ey_v eyb_fixed_checked(int op, int bits, bool sign, ey_v a, ey_v b, const ey_type *opt);
ey_v eyb_fixed_from(ey_v n, int bits, bool sign, const ey_type *opt);
ey_v eyb_fixed_to_int(int bits, bool sign, ey_v a);

/* enums */
ey_v eyb_enum_variants(const ey_type *t, const ey_type *list);
ey_v eyb_enum_parse(ey_v name, const ey_type *opt);

/* faults and output */
_Noreturn void ey_unreachable(const char *at, ey_v why);
_Noreturn void ey_expect_failed(const char *at, ey_v src, ey_v lsrc, const ey_type *lt, ey_v l);
void ey_eprintln(ey_v s);
ey_v eyb_env_get(ey_v name, const ey_type *opt);

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

/* fs and process */
void ey_sandbox(bool on);
void ey_sandbox_end(void);
ey_v eyb_fs_read_text(ey_v p, const ey_type *res);
ey_v eyb_fs_write_text(ey_v p, ey_v text, bool append, const ey_type *res);
bool eyb_fs_exists(ey_v p);
ey_v eyb_fs_list(ey_v p, const ey_type *res);
ey_v eyb_fs_make_dir(ey_v p, const ey_type *res);
ey_v eyb_fs_remove(ey_v p, const ey_type *res);
ey_v eyb_process_run(ey_v program, ey_v args, ey_v input, ey_v dir, const ey_type *res);

#endif
