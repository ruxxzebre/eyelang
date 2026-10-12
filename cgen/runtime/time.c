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
 * local seconds, with Go's choice in a gap or an overlap: in Dublin, 01:30
 * on the day the clocks go forward is 02:30, and on the day they go back
 * the second 01:30. */
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

/* ---------------------------------------------------------------- the time methods (SPECS.md §20) */

/* local_secs is the clock in z at the instant: seconds since 1970 as the
 * clock shows them. */
static int64_t local_secs(const ey_instant *t) {
	int64_t unix = floor_div(t->ns, 1000000000), start, end;
	return unix + lookup(t->zone, unix, &start, &end);
}

/* at.unix(): whole seconds since 1970-01-01 00:00 UTC */
ey_v eyb_instant_unix(ey_v i) { return ey_int_i64(floor_div(((ey_instant *)(uintptr_t)i)->ns, 1000000000)); }

/* at.date(in: zone) and at.time(in: zone) */
static int64_t clock_in(const char *at, ey_v i, ey_v zone) {
	ey_instant t = *(ey_instant *)(uintptr_t)i;
	t.zone = ey_zone_find(at, zone);
	return local_secs(&t);
}
int64_t eyb_instant_date(const char *at, ey_v i, ey_v zone) { return floor_div(clock_in(at, i, zone), 86400); }
int32_t eyb_instant_time(const char *at, ey_v i, ey_v zone) {
	int64_t local = clock_in(at, i, zone);
	return (int32_t)(local - floor_div(local, 86400) * 86400);
}

/* t.next(in: zone, after: instant): the first instant after `after` when
 * the clock in zone shows t, on after's day there or the next */
ey_v eyb_time_next(const char *at, int32_t secs, ey_v zone, ey_v after) {
	const ey_zone *z = ey_zone_find(at, zone);
	ey_instant a = *(ey_instant *)(uintptr_t)after;
	a.zone = z;
	int64_t day = floor_div(local_secs(&a), 86400);
	int64_t t = local_to_unix(z, day * 86400 + secs);
	if (t * 1000000000 <= a.ns) t = local_to_unix(z, (day + 1) * 86400 + secs);
	return ey_instant_new(t * 1000000000, z);
}

/* day.at(t, in: zone) */
ey_v eyb_date_at(const char *at, int64_t day, int32_t secs, ey_v zone) {
	return ey_instant_local(at, day, secs, zone);
}

static const char *const month_names[] = {"January", "February", "March",     "April",   "May",      "June",
                                          "July",    "August",   "September", "October", "November", "December"};
static const char *const weekday_names[] = {"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"};

/* strftime's directives, as the interpreter's formatDate has them: %A
 * weekday, %B month, %d day, %m month number, %Y year; %H hour from 00 to
 * 23, %I hour from 01 to 12, %M minute, %S second, %p AM or PM; %% a
 * percent sign. A - after the % drops the leading zero: %-d, %-H, %-I. Any
 * other character after a % is written without the %. */
static ey_v format_clock(int64_t day, int64_t secs, ey_v pattern) {
	int64_t y, m, d;
	ey_civil_from_days(day, &y, &m, &d);
	int64_t h = secs / 3600, mi = secs / 60 % 60, s = secs % 60, h12 = h % 12 == 0 ? 12 : h % 12;
	int64_t wd = ((day % 7) + 11) % 7; /* 1970-01-01 was a Thursday */
	ey_str *p = EY_STR(pattern);
	ey_buf b = {0};
	for (int64_t i = 0; i < p->len; i++) {
		char c = p->b[i];
		if (c != '%' || i + 1 == p->len) {
			ey_buf_char(&b, c);
			continue;
		}
		c = p->b[++i];
		switch (c) {
		case 'A': ey_buf_str(&b, weekday_names[wd]); break;
		case 'B': ey_buf_str(&b, month_names[m - 1]); break;
		case 'd': ey_buf_u64(&b, (uint64_t)d, 2); break;
		case 'm': ey_buf_u64(&b, (uint64_t)m, 2); break;
		case 'Y': ey_buf_i64(&b, y); break;
		case 'H': ey_buf_u64(&b, (uint64_t)h, 2); break;
		case 'I': ey_buf_u64(&b, (uint64_t)h12, 2); break;
		case 'M': ey_buf_u64(&b, (uint64_t)mi, 2); break;
		case 'S': ey_buf_u64(&b, (uint64_t)s, 2); break;
		case 'p': ey_buf_str(&b, h < 12 ? "AM" : "PM"); break;
		case '-':
			if (i + 1 == p->len) break;
			switch (p->b[i + 1]) {
			case 'd': i++; ey_buf_u64(&b, (uint64_t)d, 0); break;
			case 'H': i++; ey_buf_u64(&b, (uint64_t)h, 0); break;
			case 'I': i++; ey_buf_u64(&b, (uint64_t)h12, 0); break;
			}
			break;
		default: ey_buf_char(&b, c);
		}
	}
	return ey_buf_take(&b);
}

/* day.format(pattern) and t.format(pattern); a time of day is on day 0,
 * 0000-01-01, as the interpreter has it */
ey_v eyb_date_format(int64_t day, ey_v pattern) { return format_clock(day, 0, pattern); }
ey_v eyb_time_format(int32_t secs, ey_v pattern) { return format_clock(ey_days_from_civil(0, 1, 1), secs, pattern); }

/* whether a range of times of day holds t: one whose end comes before its
 * start wraps midnight, so 23:00..08:00 holds 23:30 and 07:00 */
bool eyb_time_range_contains(const char *at, ey_v range, int32_t t) {
	ey_range_obj *r = EY_RANGEP(range);
	if (r->flags & (EY_RANGE_NO_START | EY_RANGE_NO_END)) ey_unsupported(at, "contains on a range of times without both ends");
	int64_t s = (int64_t)r->start, e = (int64_t)r->end, v = t;
	bool after_end = v > e || (v == e && !(r->flags & EY_RANGE_INCLUSIVE));
	if (s <= e) return v >= s && !after_end;
	return v >= s || !after_end;
}
