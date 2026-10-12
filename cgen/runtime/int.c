/* Int: a small integer in the word, or a big integer on the heap.
 *
 * Big integers are written here, not vendored: a sign and a magnitude of
 * 32-bit limbs, with schoolbook multiplication and bit-by-bit division,
 * which is plenty for the sizes programs reach. A result that fits the
 * small range is always made small, so a big integer is never in it. */
#include <math.h>
#include <stdio.h>
#include <stdlib.h>

#include "eye.h"

/* A magnitude and a sign, read from either kind of Int. */
typedef struct {
	int neg;
	uint32_t n;
	const uint32_t *d;
	uint32_t small[2];
} bigv;

static void view(ey_v v, bigv *b) {
	if (EY_IS_SMALL(v)) {
		int64_t x = EY_SMALL_VAL(v);
		uint64_t u = x < 0 ? (uint64_t)0 - (uint64_t)x : (uint64_t)x;
		b->neg = x < 0;
		b->small[0] = (uint32_t)u;
		b->small[1] = (uint32_t)(u >> 32);
		b->n = b->small[1] ? 2 : b->small[0] ? 1 : 0;
		b->d = b->small;
		return;
	}
	ey_big *g = (ey_big *)(uintptr_t)v;
	b->neg = g->neg;
	b->n = g->n;
	b->d = g->d;
}

static uint32_t *limbs(size_t n) {
	uint32_t *d = calloc(n ? n : 1, sizeof(uint32_t));
	if (!d) {
		fputs("out of memory\n", stderr);
		exit(2);
	}
	return d;
}

ey_v ey_int_big(int neg, const uint32_t *d, uint32_t n) {
	while (n > 0 && d[n - 1] == 0) n--;
	if (n == 0) return EY_SMALL(0);
	if (n <= 2) {
		uint64_t u = d[0] | (n == 2 ? (uint64_t)d[1] << 32 : 0);
		if (!neg && u <= (uint64_t)EY_SMALL_MAX) return EY_SMALL((int64_t)u);
		if (neg && u <= (uint64_t)EY_SMALL_MAX + 1) return EY_SMALL((int64_t)(0 - u));
	}
	ey_big *g = ey_alloc(sizeof(ey_big) + n * sizeof(uint32_t), &ey_t_big);
	g->neg = neg;
	g->n = n;
	memcpy(g->d, d, n * sizeof(uint32_t));
	return EY_V(g);
}

ey_v ey_int_i64(int64_t x) {
	if (x >= EY_SMALL_MIN && x <= EY_SMALL_MAX) return EY_SMALL(x);
	uint64_t u = x < 0 ? (uint64_t)0 - (uint64_t)x : (uint64_t)x;
	uint32_t d[2] = {(uint32_t)u, (uint32_t)(u >> 32)};
	return ey_int_big(x < 0, d, 2);
}

ey_v ey_int_u64(uint64_t u) {
	uint32_t d[2] = {(uint32_t)u, (uint32_t)(u >> 32)};
	return ey_int_big(0, d, 2);
}

/* ---------------------------------------------------------------- magnitudes */

static int mag_cmp(const uint32_t *a, uint32_t an, const uint32_t *b, uint32_t bn) {
	if (an != bn) return an < bn ? -1 : 1;
	for (uint32_t i = an; i-- > 0;)
		if (a[i] != b[i]) return a[i] < b[i] ? -1 : 1;
	return 0;
}

/* out has max(an, bn) + 1 limbs. */
static void mag_add(const uint32_t *a, uint32_t an, const uint32_t *b, uint32_t bn, uint32_t *out) {
	uint64_t carry = 0;
	uint32_t n = an > bn ? an : bn;
	for (uint32_t i = 0; i < n; i++) {
		uint64_t s = carry + (i < an ? a[i] : 0) + (i < bn ? b[i] : 0);
		out[i] = (uint32_t)s;
		carry = s >> 32;
	}
	out[n] = (uint32_t)carry;
}

/* a >= b; out has an limbs. */
static void mag_sub(const uint32_t *a, uint32_t an, const uint32_t *b, uint32_t bn, uint32_t *out) {
	int64_t borrow = 0;
	for (uint32_t i = 0; i < an; i++) {
		int64_t s = (int64_t)a[i] - (i < bn ? b[i] : 0) - borrow;
		borrow = s < 0;
		out[i] = (uint32_t)(s + (borrow ? ((int64_t)1 << 32) : 0));
	}
}

/* out has an + bn limbs, zeroed. */
static void mag_mul(const uint32_t *a, uint32_t an, const uint32_t *b, uint32_t bn, uint32_t *out) {
	for (uint32_t i = 0; i < an; i++) {
		uint64_t carry = 0;
		for (uint32_t j = 0; j < bn; j++) {
			uint64_t t = (uint64_t)a[i] * b[j] + out[i + j] + carry;
			out[i + j] = (uint32_t)t;
			carry = t >> 32;
		}
		out[i + bn] += (uint32_t)carry;
	}
}

/* q (an limbs, zeroed) and r (bn + 1 limbs, zeroed) of a / b, b not zero. */
static void mag_divmod(const uint32_t *a, uint32_t an, const uint32_t *b, uint32_t bn, uint32_t *q, uint32_t *r) {
	if (bn == 1) {
		uint64_t rem = 0;
		for (uint32_t i = an; i-- > 0;) {
			uint64_t cur = (rem << 32) | a[i];
			q[i] = (uint32_t)(cur / b[0]);
			rem = cur % b[0];
		}
		r[0] = (uint32_t)rem;
		return;
	}
	uint32_t rn = bn + 1;
	for (uint64_t bit = (uint64_t)an * 32; bit-- > 0;) {
		/* r = r << 1 | the bit */
		uint32_t carry = (a[bit / 32] >> (bit % 32)) & 1;
		for (uint32_t i = 0; i < rn; i++) {
			uint32_t next = r[i] >> 31;
			r[i] = (r[i] << 1) | carry;
			carry = next;
		}
		uint32_t used = rn;
		while (used > 0 && r[used - 1] == 0) used--;
		if (mag_cmp(r, used, b, bn) >= 0) {
			mag_sub(r, used, b, bn, r);
			q[bit / 32] |= (uint32_t)1 << (bit % 32);
		}
	}
}

/* ---------------------------------------------------------------- arithmetic */

static ey_v add_signed(const bigv *a, const bigv *b, int bneg) {
	uint32_t n = (a->n > b->n ? a->n : b->n) + 1;
	uint32_t *out = limbs(n);
	ey_v r;
	if (a->neg == bneg) {
		mag_add(a->d, a->n, b->d, b->n, out);
		r = ey_int_big(a->neg, out, n);
	} else if (mag_cmp(a->d, a->n, b->d, b->n) >= 0) {
		mag_sub(a->d, a->n, b->d, b->n, out);
		r = ey_int_big(a->neg, out, a->n);
	} else {
		mag_sub(b->d, b->n, a->d, a->n, out);
		r = ey_int_big(bneg, out, b->n);
	}
	free(out);
	return r;
}

ey_v ey_int_add_slow(ey_v x, ey_v y) {
	bigv a, b;
	view(x, &a);
	view(y, &b);
	return add_signed(&a, &b, b.neg);
}

ey_v ey_int_sub_slow(ey_v x, ey_v y) {
	bigv a, b;
	view(x, &a);
	view(y, &b);
	return add_signed(&a, &b, !b.neg);
}

ey_v ey_int_mul_slow(ey_v x, ey_v y) {
	bigv a, b;
	view(x, &a);
	view(y, &b);
	uint32_t *out = limbs(a.n + b.n);
	mag_mul(a.d, a.n, b.d, b.n, out);
	ey_v r = ey_int_big(a.neg != b.neg, out, a.n + b.n);
	free(out);
	return r;
}

ey_v ey_int_neg(ey_v x) {
	if (EY_IS_SMALL(x)) return ey_int_i64(-EY_SMALL_VAL(x));
	bigv a;
	view(x, &a);
	return ey_int_big(!a.neg, a.d, a.n);
}

/* floor division and its remainder, which has the divisor's sign */
static void divmod(const char *at, ey_v x, ey_v y, ey_v *qo, ey_v *ro) {
	if (EY_IS_SMALL(x) && EY_IS_SMALL(y)) {
		int64_t a = EY_SMALL_VAL(x), b = EY_SMALL_VAL(y);
		if (b == 0) ey_fault(at, "division by zero");
		int64_t q = a / b, r = a % b;
		if (r != 0 && ((r < 0) != (b < 0))) {
			q--;
			r += b;
		}
		if (qo) *qo = ey_int_i64(q);
		if (ro) *ro = ey_int_i64(r);
		return;
	}
	bigv a, b;
	view(x, &a);
	view(y, &b);
	if (b.n == 0) ey_fault(at, "division by zero");
	uint32_t *q = limbs(a.n + 1), *r = limbs(b.n + 1);
	if (a.n > 0) mag_divmod(a.d, a.n, b.d, b.n, q, r);
	uint32_t rn = b.n + 1;
	while (rn > 0 && r[rn - 1] == 0) rn--;
	if (rn != 0 && a.neg != b.neg) {
		/* q = -(|q| + 1), r = |b| - |r| with b's sign */
		uint32_t one = 1;
		uint32_t *q1 = limbs(a.n + 2);
		mag_add(q, a.n, &one, 1, q1);
		uint32_t *r1 = limbs(b.n);
		mag_sub(b.d, b.n, r, rn, r1);
		if (qo) *qo = ey_int_big(1, q1, a.n + 1);
		if (ro) *ro = ey_int_big(b.neg, r1, b.n);
		free(q1);
		free(r1);
	} else {
		if (qo) *qo = ey_int_big(a.neg != b.neg, q, a.n);
		if (ro) *ro = ey_int_big(a.neg, r, rn);
	}
	free(q);
	free(r);
}

ey_v ey_int_div_slow(const char *at, ey_v a, ey_v b) {
	ey_v q;
	divmod(at, a, b, &q, NULL);
	return q;
}

ey_v ey_int_mod_slow(const char *at, ey_v a, ey_v b) {
	ey_v r;
	divmod(at, a, b, NULL, &r);
	return r;
}

int ey_int_cmp(ey_v x, ey_v y) {
	if (EY_IS_SMALL(x) && EY_IS_SMALL(y)) return (int64_t)x < (int64_t)y ? -1 : (int64_t)x > (int64_t)y;
	bigv a, b;
	view(x, &a);
	view(y, &b);
	if (a.neg != b.neg) return a.neg ? -1 : 1;
	int c = mag_cmp(a.d, a.n, b.d, b.n);
	return a.neg ? -c : c;
}

bool ey_int_eq(ey_v x, ey_v y) {
	if (EY_IS_SMALL(x) || EY_IS_SMALL(y)) return x == y; /* a big one is never small */
	return ey_int_cmp(x, y) == 0;
}

/* ---------------------------------------------------------------- conversions */

double ey_int_to_f64(ey_v x) {
	if (EY_IS_SMALL(x)) return (double)EY_SMALL_VAL(x);
	bigv a;
	view(x, &a);
	/* The top 64 bits, with any bit below them kept as the lowest bit, so
	 * the conversion rounds as an exact one would. */
	uint32_t n = a.n;
	int shift = 0;
	uint64_t top;
	if (n <= 2) {
		top = a.d[0] | (n == 2 ? (uint64_t)a.d[1] << 32 : 0);
	} else {
		int lead = __builtin_clz(a.d[n - 1]);
		/* bits n-1..: take 64 bits starting at the leading one */
		unsigned __int128 w = ((unsigned __int128)a.d[n - 1] << 64) | ((unsigned __int128)a.d[n - 2] << 32) | a.d[n - 3];
		w <<= lead; /* the leading one at bit 95 */
		top = (uint64_t)(w >> 32);
		bool sticky = (uint32_t)w != 0;
		for (uint32_t i = 0; i + 3 < n && !sticky; i++) sticky = a.d[i] != 0;
		if (sticky) top |= 1;
		shift = (int)(n - 3) * 32 + 32 - lead;
	}
	double d = ldexp((double)top, shift);
	return a.neg ? -d : d;
}

bool ey_int_from_f64(double d, ey_v *out) {
	if (isnan(d) || isinf(d)) return false;
	d = trunc(d);
	if (d >= -4611686018427387904.0 && d < 4611686018427387904.0) {
		*out = EY_SMALL((int64_t)d);
		return true;
	}
	int exp;
	double m = frexp(fabs(d), &exp); /* d = m * 2^exp, m in [0.5, 1) */
	uint64_t mant = (uint64_t)ldexp(m, 53);
	int shift = exp - 53; /* positive here */
	uint32_t n = (uint32_t)(shift / 32) + 3;
	uint32_t *dg = limbs(n);
	unsigned __int128 w = (unsigned __int128)mant << (shift % 32);
	dg[shift / 32] = (uint32_t)w;
	dg[shift / 32 + 1] = (uint32_t)(w >> 32);
	dg[shift / 32 + 2] = (uint32_t)(w >> 64);
	*out = ey_int_big(d < 0, dg, n);
	free(dg);
	return true;
}

bool ey_int_fits(ey_v x, int64_t lo, int64_t hi, bool unsigned64) {
	if (EY_IS_SMALL(x)) {
		int64_t v = EY_SMALL_VAL(x);
		if (unsigned64) return v >= 0;
		return v >= lo && v <= hi;
	}
	bigv a;
	view(x, &a);
	if (a.n > 2) return false;
	uint64_t u = a.d[0] | (a.n == 2 ? (uint64_t)a.d[1] << 32 : 0);
	if (unsigned64) return !a.neg;
	if (a.neg) return lo < 0 && u <= (uint64_t)0 - (uint64_t)lo;
	return hi >= 0 && u <= (uint64_t)hi;
}

uint64_t ey_int_bits(ey_v x) {
	if (EY_IS_SMALL(x)) return (uint64_t)EY_SMALL_VAL(x);
	bigv a;
	view(x, &a);
	uint64_t u = a.d[0] | (a.n >= 2 ? (uint64_t)a.d[1] << 32 : 0);
	return a.neg ? (uint64_t)0 - u : u;
}

bool ey_int_i64_of(ey_v x, int64_t *out) {
	if (EY_IS_SMALL(x)) {
		*out = EY_SMALL_VAL(x);
		return true;
	}
	if (!ey_int_fits(x, INT64_MIN, INT64_MAX, false)) return false;
	*out = (int64_t)ey_int_bits(x);
	return true;
}

void ey_int_write(ey_buf *b, ey_v x) {
	if (EY_IS_SMALL(x)) {
		ey_buf_i64(b, EY_SMALL_VAL(x));
		return;
	}
	bigv a;
	view(x, &a);
	uint32_t n = a.n;
	uint32_t *cur = limbs(n), *q = limbs(n);
	memcpy(cur, a.d, n * sizeof(uint32_t));
	/* chunks of nine digits, least significant first */
	uint32_t *chunks = limbs((size_t)n * 2 + 1);
	size_t nc = 0;
	while (n > 0) {
		uint32_t ten9 = 1000000000, r[2] = {0, 0};
		memset(q, 0, n * sizeof(uint32_t));
		mag_divmod(cur, n, &ten9, 1, q, r);
		chunks[nc++] = r[0];
		memcpy(cur, q, n * sizeof(uint32_t));
		while (n > 0 && cur[n - 1] == 0) n--;
	}
	if (a.neg) ey_buf_char(b, '-');
	ey_buf_u64(b, chunks[nc - 1], 0);
	for (size_t i = nc - 1; i-- > 0;) ey_buf_u64(b, chunks[i], 9);
	free(cur);
	free(q);
	free(chunks);
}

/* ey_int_parse reads an integer in base 2 to 36, as Go's big.Int.SetString
 * does with that base: an optional sign, then at least one digit. */
bool ey_int_parse(const char *s, int64_t len, int base, ey_v *out) {
	int64_t i = 0;
	int neg = 0;
	if (i < len && (s[i] == '+' || s[i] == '-')) neg = s[i++] == '-';
	if (i == len) return false;
	uint32_t cap = (uint32_t)((len - i) * 6 / 32) + 2, n = 0;
	uint32_t *d = limbs(cap);
	for (; i < len; i++) {
		char c = s[i];
		int digit = c >= '0' && c <= '9' ? c - '0' : c >= 'a' && c <= 'z' ? c - 'a' + 10 : c >= 'A' && c <= 'Z' ? c - 'A' + 10 : 99;
		if (digit >= base) {
			free(d);
			return false;
		}
		uint64_t carry = (uint64_t)digit;
		for (uint32_t j = 0; j < n; j++) {
			uint64_t t = (uint64_t)d[j] * (uint64_t)base + carry;
			d[j] = (uint32_t)t;
			carry = t >> 32;
		}
		if (carry) d[n++] = (uint32_t)carry;
	}
	*out = ey_int_big(neg, d, n);
	free(d);
	return true;
}

ey_v eyb_int_div_rem(ey_v a, ey_v b, bool rem, const ey_type *opt) {
	if (b == EY_SMALL(0)) return ey_none(opt);
	return ey_some(opt, rem ? ey_int_mod("<rem>", a, b) : ey_int_div("<div>", a, b));
}
