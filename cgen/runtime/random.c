/* The random module's primitives (check/std/random.eye), drawing the same
 * numbers as the interpreter's (interp/system.go): Go's math/rand/v2 over a
 * PCG source, with its algorithms for a number below n, a Float and a
 * shuffle. A test starts from the seed the interpreter's tests use, (1, 2),
 * so a test sees the same numbers natively and under eye test; a program
 * seeds it from the system. */
#include <stdio.h>
#include <stdlib.h>
#include <time.h>

#include "eye.h"
#include "builtins.h"

static uint64_t rnd_hi, rnd_lo; /* the PCG's 128-bit state */
static bool rnd_seeded;

void ey_random_seed(uint64_t hi, uint64_t lo) {
	rnd_hi = hi;
	rnd_lo = lo;
	rnd_seeded = true;
}

void ey_random_test(void) { ey_random_seed(1, 2); }

static uint64_t rnd_mix(uint64_t x) { /* splitmix64's finaliser */
	x += 0x9e3779b97f4a7c15u;
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9u;
	x = (x ^ (x >> 27)) * 0x94d049bb133111ebu;
	return x ^ (x >> 31);
}

/* seeds from /dev/urandom, or where there is none, from the clock and an
 * address */
static void rnd_seed_system(void) {
	uint64_t s[2] = {0, 0};
	bool got = false;
#ifndef _WIN32
	FILE *f = fopen("/dev/urandom", "rb");
	if (f) {
		got = fread(s, sizeof s, 1, f) == 1;
		fclose(f);
	}
#endif
	if (!got) {
		uint64_t t = (uint64_t)time(NULL) ^ ((uint64_t)clock() << 32) ^ (uint64_t)(uintptr_t)&s;
		s[0] = rnd_mix(t);
		s[1] = rnd_mix(s[0] ^ (uint64_t)(uintptr_t)&rnd_seed_system);
	}
	ey_random_seed(s[0], s[1]);
}

/* Go's PCG.Uint64: the state times a 128-bit multiplier plus an increment,
 * then DXSM */
static uint64_t rnd_u64(void) {
	if (!rnd_seeded) rnd_seed_system();
	const uint64_t mul_hi = 2549297995355413924u, mul_lo = 4865540595714422341u;
	const uint64_t inc_hi = 6364136223846793005u, inc_lo = 1442695040888963407u;
	unsigned __int128 m = (unsigned __int128)rnd_lo * mul_lo;
	uint64_t hi = (uint64_t)(m >> 64) + rnd_hi * mul_lo + rnd_lo * mul_hi;
	uint64_t lo = (uint64_t)m;
	uint64_t nlo = lo + inc_lo;
	hi += inc_hi + (nlo < lo);
	lo = nlo;
	rnd_lo = lo;
	rnd_hi = hi;
	const uint64_t cheap = 0xda942042e4dd58b5u;
	hi ^= hi >> 32;
	hi *= cheap;
	hi ^= hi >> 48;
	hi *= (lo | 1);
	return hi;
}

/* Go's Rand.uint64n: a number below n, each as likely, by Lemire's method */
static uint64_t rnd_below(uint64_t n) {
	if ((n & (n - 1)) == 0) return rnd_u64() & (n - 1);
	unsigned __int128 p = (unsigned __int128)rnd_u64() * n;
	if ((uint64_t)p < n) {
		uint64_t thresh = (0 - n) % n;
		while ((uint64_t)p < thresh) p = (unsigned __int128)rnd_u64() * n;
	}
	return (uint64_t)(p >> 64);
}

/* random.int(range): None for an empty range */
void eyb_random_int(const char *at, ey_v range, void *out, const ey_type *opt) {
	ey_range_obj *r = EY_RANGEP(range);
	int64_t lo, hi;
	if ((r->flags & (EY_RANGE_NO_START | EY_RANGE_NO_END)) || !ey_int_i64_of(r->start, &lo) || !ey_int_i64_of(r->end, &hi))
		ey_fault(at, "random.int takes a range of Ints that fit in 64 bits");
	if (!(r->flags & EY_RANGE_INCLUSIVE)) hi = (int64_t)((uint64_t)hi - 1);
	if (hi < lo) {
		ey_none(opt, out);
		return;
	}
	uint64_t span = (uint64_t)hi - (uint64_t)lo, n;
	if (span == UINT64_MAX)
		n = rnd_u64();
	else
		n = (uint64_t)lo + rnd_below(span + 1);
	ey_some_v(opt, out, ey_int_i64((int64_t)n));
}

/* random.float(): from 0 up to, but not including, 1 */
double eyb_random_float(void) { return (double)(rnd_u64() << 11 >> 11) / (double)(UINT64_C(1) << 53); }

/* random.pick(xs): None for no elements */
void eyb_random_pick(ey_v xs, void *out, const ey_type *opt) {
	ey_list *l = EY_LIST(xs);
	if (l->len == 0) {
		ey_none(opt, out);
		return;
	}
	void *p = ey_list_item(l, (int64_t)rnd_below((uint64_t)l->len));
	ey_dup_at(l->h.ty->args[0], p);
	ey_some(opt, out, p);
}

/* random.shuffle(xs): a new list, by Go's Fisher-Yates */
ey_v eyb_random_shuffle(ey_v xs) {
	ey_list *l = EY_LIST(xs);
	const ey_type *et = l->h.ty->args[0];
	ey_v out = ey_list_new(l->h.ty, l->len, l->items);
	ey_list *o = EY_LIST(out);
	ey_tmp t;
	for (int64_t i = o->len - 1; i > 0; i--) {
		int64_t j = (int64_t)rnd_below((uint64_t)i + 1);
		void *a = ey_list_item(o, i), *b = ey_list_item(o, j);
		memcpy(t.b, a, et->size);
		memcpy(a, b, et->size);
		memcpy(b, t.b, et->size);
	}
	return out;
}
