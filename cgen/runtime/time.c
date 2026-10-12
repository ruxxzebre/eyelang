/* Dates, instants and time zones. A zone's offsets come from a table the
 * back end writes into the program for each zone it names, made from Go's
 * time zone database when the program is built; an Instant keeps its zone,
 * which it shows in. */
#include <stdlib.h>

#include "eye.h"
#include "builtins.h"

/* Howard Hinnant's algorithms, for the proleptic Gregorian calendar. */
int64_t ey_days_from_civil(int64_t y, int64_t m, int64_t d) {
	y -= m <= 2;
	int64_t era = (y >= 0 ? y : y - 399) / 400;
	int64_t yoe = y - era * 400;
	int64_t doy = (153 * (m + (m > 2 ? -3 : 9)) + 2) / 5 + d - 1;
	int64_t doe = yoe * 365 + yoe / 4 - yoe / 100 + doy;
	return era * 146097 + doe - 719468;
}

void ey_civil_from_days(int64_t z, int64_t *y, int64_t *m, int64_t *d) {
	z += 719468;
	int64_t era = (z >= 0 ? z : z - 146096) / 146097;
	int64_t doe = z - era * 146097;
	int64_t yoe = (doe - doe / 1460 + doe / 36524 - doe / 146096) / 365;
	int64_t doy = doe - (365 * yoe + yoe / 4 - yoe / 100);
	int64_t mp = (5 * doy + 2) / 153;
	*d = doy - (153 * mp + 2) / 5 + 1;
	*m = mp < 10 ? mp + 3 : mp - 9;
	*y = yoe + era * 400 + (*m <= 2);
}

static int64_t floor_div(int64_t a, int64_t b) {
	int64_t q = a / b;
	if ((a % b != 0) && ((a < 0) != (b < 0))) q--;
	return q;
}

/* ---------------------------------------------------------------- zones */

static const ey_zone *const *zones;
static int nzones;

void ey_zones(const ey_zone *const *z, int n) {
	zones = z;
	nzones = n;
}

const ey_zone *ey_zone_find(const char *at, ey_v name) {
	ey_str *s = EY_STR(name);
	if (s->len == 0 || strcmp(s->b, "UTC") == 0) return NULL;
	for (int i = 0; i < nzones; i++)
		if (strcmp(zones[i]->name, s->b) == 0) return zones[i];
	ey_faultf(at, "unknown time zone \"%s\": the program names no such zone where it was built", s->b);
}

/* the offset in force at unix seconds, and the seconds it is in force for */
static int32_t lookup(const ey_zone *z, int64_t unix, int64_t *start, int64_t *end) {
	*start = INT64_MIN;
	*end = INT64_MAX;
	if (!z) return 0;
	int32_t lo = 0, hi = z->n; /* the first transition after unix */
	while (lo < hi) {
		int32_t mid = (lo + hi) / 2;
		if (z->at[mid] <= unix)
			lo = mid + 1;
		else
			hi = mid;
	}
	if (lo < z->n) *end = z->at[lo];
	if (lo == 0) return z->first;
	*start = z->at[lo - 1];
	return z->offset[lo - 1];
}

ey_v ey_instant_new(int64_t ns, const ey_zone *z) {
	ey_instant *t = ey_alloc(sizeof(ey_instant), &ey_t_instant);
	t->ns = ns;
	t->zone = z;
	return EY_V(t);
}

/* local_to_unix is Go's time.Date: the instant the clock in z shows
 * local seconds, an hour after a gap and the first of an overlap. */
static int64_t local_to_unix(const ey_zone *z, int64_t local) {
	int64_t start, end;
	int32_t off = lookup(z, local, &start, &end);
	if (off != 0) {
		int64_t utc = local - off;
		if (utc < start || utc >= end) off = lookup(z, utc, &start, &end);
	}
	return local - off;
}

ey_v ey_instant_local(const char *at, int64_t days, int64_t secs, ey_v zone) {
	const ey_zone *z = ey_zone_find(at, zone);
	return ey_instant_new(local_to_unix(z, days * 86400 + secs) * 1000000000, z);
}

ey_v ey_instant_next(const char *at, int64_t secs, ey_v zone) {
	const ey_zone *z = ey_zone_find(at, zone);
	ey_v now = ey_now();
	int64_t ns = ((ey_instant *)(uintptr_t)now)->ns;
	ey_drop(now);
	int64_t unix = floor_div(ns, 1000000000), start, end;
	int64_t local = unix + lookup(z, unix, &start, &end);
	int64_t day = floor_div(local, 86400);
	int64_t t = local_to_unix(z, day * 86400 + secs);
	if (t * 1000000000 <= ns) t = local_to_unix(z, (day + 1) * 86400 + secs);
	return ey_instant_new(t * 1000000000, z);
}

void ey_instant_write(ey_buf *b, ey_v v) {
	ey_instant *t = (ey_instant *)(uintptr_t)v;
	int64_t unix = floor_div(t->ns, 1000000000), start, end;
	int64_t local = unix + lookup(t->zone, unix, &start, &end);
	int64_t day = floor_div(local, 86400), secs = local - day * 86400;
	int64_t y, m, d;
	ey_civil_from_days(day, &y, &m, &d);
	ey_buf_printf(b, "%04lld-%02lld-%02lld %02lld:%02lld in \"%s\"", (long long)y, (long long)m, (long long)d,
	              (long long)(secs / 3600), (long long)(secs / 60 % 60), t->zone ? t->zone->name : "UTC");
}

ey_v eyb_instant_add(ey_v i, int64_t d) {
	ey_instant *t = (ey_instant *)(uintptr_t)i;
	return ey_instant_new(t->ns + d, t->zone);
}

int64_t eyb_instant_sub(ey_v a, ey_v b) {
	return ((ey_instant *)(uintptr_t)a)->ns - ((ey_instant *)(uintptr_t)b)->ns;
}
