/* The fixed-width integers (SPECS.md §5): Byte, Int32, Int64, UInt32 and
 * UInt64, as their two's complement bits, signed ones sign-extended to 64.
 * Arithmetic is on unsigned words or with the compiler's overflow builtins,
 * so nothing here is undefined. Also enums' derived functions. */
#include <stdlib.h>

#include "eye.h"
#include "builtins.h"

static uint64_t mask(int bits) { return bits == 64 ? ~(uint64_t)0 : ((uint64_t)1 << bits) - 1; }

/* wrap keeps the low bits of x, sign-extended for a signed type */
static uint64_t wrap(int bits, bool sign, uint64_t x) {
	if (bits == 64) return x;
	uint64_t m = mask(bits);
	x &= m;
	if (sign && ((x >> (bits - 1)) & 1)) x |= ~m;
	return x;
}

static uint64_t shift_count(const char *at, const char *name, ey_v n) {
	if (!EY_IS_SMALL(n)) {
		if (ey_int_cmp(n, EY_SMALL(0)) < 0) ey_faultf(at, "%s takes a count that isn't negative", name);
		return 64;
	}
	int64_t c = EY_SMALL_VAL(n);
	if (c < 0) ey_faultf(at, "%s takes a count that isn't negative, and this is %lld", name, (long long)c);
	return c > 64 ? 64 : (uint64_t)c;
}

ey_v eyb_fixed(const char *at, int op, int bits, bool sign, ey_v a, ey_v b) {
	switch (op) {
	case EYF_WADD:
		return wrap(bits, sign, a + b);
	case EYF_WSUB:
		return wrap(bits, sign, a - b);
	case EYF_WMUL:
		return wrap(bits, sign, a * b);
	case EYF_AND:
		return wrap(bits, sign, a & b);
	case EYF_OR:
		return wrap(bits, sign, a | b);
	case EYF_XOR:
		return wrap(bits, sign, a ^ b);
	case EYF_NOT:
		return wrap(bits, sign, ~a);
	case EYF_SHL: {
		uint64_t n = shift_count(at, "shiftLeft", b);
		if (n >= (uint64_t)bits) return 0;
		return wrap(bits, sign, a << n);
	}
	case EYF_SHR: {
		uint64_t n = shift_count(at, "shiftRight", b);
		if (sign) return wrap(bits, sign, (uint64_t)((int64_t)a >> (n > 63 ? 63 : n)));
		if (n >= (uint64_t)bits) return 0;
		return (a & mask(bits)) >> n;
	}
	}
	return 0;
}

void eyb_fixed_checked(int op, int bits, bool sign, ey_v a, ey_v b, void *out, const ey_type *opt) {
	if (bits == 64) {
		if (sign) {
			int64_t r;
			bool over = op == 0 ? __builtin_add_overflow((int64_t)a, (int64_t)b, &r)
			            : op == 1 ? __builtin_sub_overflow((int64_t)a, (int64_t)b, &r)
			                      : __builtin_mul_overflow((int64_t)a, (int64_t)b, &r);
			if (over)
				ey_none(opt, out);
			else
				ey_some_v(opt, out, (uint64_t)r);
			return;
		}
		uint64_t r;
		bool over = op == 0 ? __builtin_add_overflow(a, b, &r)
		            : op == 1 ? __builtin_sub_overflow(a, b, &r)
		                      : __builtin_mul_overflow(a, b, &r);
		if (over)
			ey_none(opt, out);
		else
			ey_some_v(opt, out, r);
		return;
	}
	/* narrower types: exact in 64 bits, then checked against the bounds */
	int64_t x = sign ? (int64_t)a : (int64_t)(a & mask(bits));
	int64_t y = sign ? (int64_t)b : (int64_t)(b & mask(bits));
	int64_t r = op == 0 ? x + y : op == 1 ? x - y : x * y;
	int64_t lo = sign ? -((int64_t)1 << (bits - 1)) : 0;
	int64_t hi = sign ? ((int64_t)1 << (bits - 1)) - 1 : (int64_t)mask(bits);
	if (r < lo || r > hi)
		ey_none(opt, out);
	else
		ey_some_v(opt, out, (uint64_t)r);
}

void eyb_fixed_from(ey_v n, int bits, bool sign, void *out, const ey_type *opt) {
	bool fits;
	if (bits == 64 && !sign)
		fits = ey_int_fits(n, 0, 0, true);
	else if (bits == 64)
		fits = ey_int_fits(n, INT64_MIN, INT64_MAX, false);
	else if (sign)
		fits = ey_int_fits(n, -((int64_t)1 << (bits - 1)), ((int64_t)1 << (bits - 1)) - 1, false);
	else
		fits = ey_int_fits(n, 0, (int64_t)mask(bits), false);
	if (!fits)
		ey_none(opt, out);
	else
		ey_some_v(opt, out, ey_int_bits(n));
}

ey_v eyb_fixed_to_int(int bits, bool sign, ey_v a) {
	if (sign) return ey_int_i64((int64_t)a);
	if (bits == 64) return ey_int_u64(a);
	return EY_SMALL((int64_t)(a & mask(bits)));
}

/* ---------------------------------------------------------------- enums */

/* the variants of an enum, by their numbers */
ey_v eyb_enum_variants(const ey_type *t, const ey_type *list) {
	char *items = malloc((size_t)t->size * (t->n ? t->n : 1));
	if (!items) ey_out_of_memory();
	for (uint32_t i = 0; i < t->n; i++) ey_variant_make(t, items + i * t->size, i, NULL);
	return ey_list_take(list, t->n, items);
}

void eyb_enum_parse(ey_v name, void *out, const ey_type *opt) {
	const ey_type *t = opt->variants[0].fields[0];
	for (uint32_t i = 0; i < t->n; i++) {
		if (strcmp(t->variants[i].name, EY_STR(name)->b) == 0 && (int64_t)strlen(t->variants[i].name) == EY_STR(name)->len) {
			ey_tmp v;
			ey_variant_make(t, v.b, i, NULL);
			ey_some(opt, out, v.b);
			return;
		}
	}
	ey_none(opt, out);
}
