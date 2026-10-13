/* toJson: a value in its JSON shape (SPECS.md §17, "Values as JSON"), byte
 * for byte as the interpreter writes it (interp/json.go's toWire, then
 * Go's encoding/json):
 *
 *   - a record is an object, and so is a variant with fields, tagged with
 *     "kind"; a positional field is "value", or "value1", "value2", …;
 *   - an object's keys are in byte order, as encoding/json sorts a map's;
 *   - a type whose variants have no fields is a string, the variant's name;
 *   - Some(x) is x and None null; a type that wraps one value is that value
 *     (the back end holds it as that value already);
 *   - lists, sets, tuples and Bytes are arrays, and a map whose keys are all
 *     strings in JSON is an object, any other map an array of [key, value]
 *     pairs;
 *   - an Instant is RFC 3339 in its zone, a Date and a Time as they show, a
 *     Duration a whole number of milliseconds, a Size of bytes;
 *   - strings are escaped as encoding/json escapes them: <, > and & too, and
 *     U+2028 and U+2029; an invalid byte becomes U+FFFD;
 *   - a Float is written as encoding/json writes a float64, and NaN or an
 *     infinity is a fault.
 *
 * It walks the value by its type's descriptor. Every static name here
 * starts with json_, since at -O2 the runtime is one translation unit. */
#include <math.h>
#include <stdio.h>
#include <stdlib.h>

#include "eye.h"
#include "builtins.h"

static void json_value(ey_buf *b, const ey_type *t, const void *p, const char *at);

/* ---------------------------------------------------------------- strings */

static void json_string(ey_buf *b, const char *s, int64_t n) {
	static const char hex[] = "0123456789abcdef";
	ey_buf_char(b, '"');
	int64_t start = 0;
	for (int64_t i = 0; i < n;) {
		unsigned char c = (unsigned char)s[i];
		if (c < 0x80) {
			if (c >= 0x20 && c != '"' && c != '\\' && c != '<' && c != '>' && c != '&') {
				i++;
				continue;
			}
			ey_buf_add(b, s + start, i - start);
			switch (c) {
			case '"': ey_buf_str(b, "\\\""); break;
			case '\\': ey_buf_str(b, "\\\\"); break;
			case '\b': ey_buf_str(b, "\\b"); break;
			case '\f': ey_buf_str(b, "\\f"); break;
			case '\n': ey_buf_str(b, "\\n"); break;
			case '\r': ey_buf_str(b, "\\r"); break;
			case '\t': ey_buf_str(b, "\\t"); break;
			default: {
				char u[6] = {'\\', 'u', '0', '0', hex[c >> 4], hex[c & 15]};
				ey_buf_add(b, u, 6);
			}
			}
			start = ++i;
			continue;
		}
		uint32_t r;
		int64_t k = ey_utf8_decode(s + i, n - i, &r);
		if (r == 0xfffd && k == 1) {
			ey_buf_add(b, s + start, i - start);
			ey_buf_str(b, "\xef\xbf\xbd");
			start = ++i;
			continue;
		}
		if (r == 0x2028 || r == 0x2029) {
			ey_buf_add(b, s + start, i - start);
			ey_buf_str(b, r == 0x2028 ? "\\u2028" : "\\u2029");
			start = i + k;
		}
		i += k;
	}
	ey_buf_add(b, s + start, n - start);
	ey_buf_char(b, '"');
}

/* ---------------------------------------------------------------- numbers */

/* json_float writes d as encoding/json writes a float64: the shortest
 * digits that read back as d, in plain form between 1e-6 and 1e21 and in
 * exponent form, without a leading zero in the exponent, outside. */
static void json_float(ey_buf *b, double d, const char *at) {
	if (isnan(d) || isinf(d))
		ey_faultf(at, "toJson: %s can't be written as JSON, which has no NaN or infinities", isnan(d) ? "NaN" : d > 0 ? "+Inf" : "-Inf");
	if (d == 0) {
		ey_buf_str(b, signbit(d) ? "-0" : "0");
		return;
	}
	char e[40];
	for (int prec = 1; prec <= 17; prec++) {
		snprintf(e, sizeof e, "%.*e", prec - 1, d);
		if (strtod(e, NULL) == d) break;
	}
	/* e is [-]d[.ddd]e±xx */
	const char *p = e;
	bool neg = *p == '-';
	if (neg) p++;
	char digits[24];
	int nd = 0;
	for (; *p && *p != 'e'; p++)
		if (*p != '.') digits[nd++] = *p;
	int exp = atoi(p + 1);
	while (nd > 1 && digits[nd - 1] == '0') nd--;
	if (neg) ey_buf_char(b, '-');
	double abs = fabs(d);
	if (abs < 1e-6 || abs >= 1e21) {
		ey_buf_char(b, digits[0]);
		if (nd > 1) {
			ey_buf_char(b, '.');
			ey_buf_add(b, digits + 1, nd - 1);
		}
		ey_buf_printf(b, "e%c%d", exp < 0 ? '-' : '+', exp < 0 ? -exp : exp);
		return;
	}
	int dp = exp + 1; /* digits before the point */
	if (dp <= 0) {
		ey_buf_str(b, "0.");
		for (int i = 0; i < -dp; i++) ey_buf_char(b, '0');
		ey_buf_add(b, digits, nd);
		return;
	}
	for (int i = 0; i < dp; i++) ey_buf_char(b, i < nd ? digits[i] : '0');
	if (nd > dp) {
		ey_buf_char(b, '.');
		ey_buf_add(b, digits + dp, nd - dp);
	}
}

/* ---------------------------------------------------------------- times */

static int64_t json_floor_div(int64_t a, int64_t b) {
	int64_t q = a / b;
	if ((a % b != 0) && ((a < 0) != (b < 0))) q--;
	return q;
}

/* the offset east of UTC, in seconds, that zone z has at unix seconds */
static int32_t json_offset(const ey_zone *z, int64_t unix) {
	if (!z) return 0;
	int32_t lo = 0, hi = z->n; /* the first transition after unix */
	while (lo < hi) {
		int32_t mid = (lo + hi) / 2;
		if (z->at[mid] <= unix)
			lo = mid + 1;
		else
			hi = mid;
	}
	return lo == 0 ? z->first : z->offset[lo - 1];
}

/* json_rfc3339 writes an Instant as Go's time.RFC3339 does, in its zone:
 * 2026-10-12T09:30:00+01:00, or Z for an offset of zero. */
static void json_rfc3339(ey_buf *b, ey_v v) {
	ey_instant *t = (ey_instant *)(uintptr_t)v;
	int64_t unix = json_floor_div(t->ns, 1000000000);
	int32_t off = json_offset(t->zone, unix);
	int64_t local = unix + off;
	int64_t day = json_floor_div(local, 86400), secs = local - day * 86400;
	int64_t y, m, d;
	ey_civil_from_days(day, &y, &m, &d);
	if (y < 0) ey_buf_char(b, '-');
	ey_buf_u64(b, (uint64_t)(y < 0 ? -y : y), 4);
	ey_buf_printf(b, "-%02lld-%02lld %02lld:%02lld:%02lld", (long long)m, (long long)d, (long long)(secs / 3600),
	              (long long)(secs / 60 % 60), (long long)(secs % 60));
	b->b[b->len - 9] = 'T';
	if (off == 0) {
		ey_buf_char(b, 'Z');
		return;
	}
	int32_t a = off < 0 ? -off : off;
	ey_buf_printf(b, "%c%02d:%02d", off < 0 ? '-' : '+', (int)(a / 3600), (int)(a / 60 % 60));
}

/* ---------------------------------------------------------------- shapes */

/* whether t is the prelude's Option: Some(x) is x, and None null */
static bool json_is_option(const ey_type *t) {
	return t->kind == EY_VARIANT && t->n == 2 && t->name && strcmp(t->name, "Option") == 0 &&
	       strcmp(t->variants[0].name, "Some") == 0 && t->variants[0].n == 1 && strcmp(t->variants[1].name, "None") == 0;
}

/* whether t's variants all have no fields, so that a value is its name */
static bool json_is_enum(const ey_type *t) {
	for (uint32_t i = 0; i < t->n; i++)
		if (t->variants[i].n > 0) return false;
	return t->n > 0;
}

/* json_raw writes the text of the value's JSON when that is a string, and
 * reports whether it is: a map whose keys are all strings is an object. */
static bool json_raw(ey_buf *b, const ey_type *t, const void *p) {
	switch (t->kind) {
	case EY_STRING: {
		ey_str *s = EY_STR(*(const ey_v *)p);
		ey_buf_add(b, s->b, s->len);
		return true;
	}
	case EY_CHAR:
		ey_buf_rune(b, *(const uint32_t *)p);
		return true;
	case EY_DATE:
	case EY_TIME:
	case EY_RANGE:
	case EY_FN:
	case EY_DYN:
	case EY_ITER:
		ey_show_to(b, t, p, false);
		return true;
	case EY_INSTANT:
		json_rfc3339(b, *(const ey_v *)p);
		return true;
	case EY_VARIANT: {
		uint32_t k = ey_variant_of(t, p);
		if (json_is_option(t)) return k == 0 && json_raw(b, t->variants[0].fields[0], ey_variant_field(t, p, 0, 0));
		if (json_is_enum(t)) {
			ey_buf_str(b, t->variants[k].name);
			return true;
		}
		return false;
	}
	}
	return false;
}

/* An object's member: its key, and the value of type t at p. */
typedef struct json_member {
	const char *key;
	int64_t len;
	const ey_type *t;
	const void *p;
} json_member;

static int json_cmp(const json_member *a, const json_member *b) {
	int64_t n = a->len < b->len ? a->len : b->len;
	int c = memcmp(a->key, b->key, (size_t)n);
	if (c) return c;
	return a->len < b->len ? -1 : a->len > b->len;
}

/* json_object writes members in their keys' byte order, as encoding/json
 * writes a map; of members with the same key, the last, as a Go map keeps
 * the last value set. */
static void json_object(ey_buf *b, json_member *ms, int64_t n, const char *at) {
	/* insertion sort, stable; objects are small */
	for (int64_t i = 1; i < n; i++) {
		json_member m = ms[i];
		int64_t j = i;
		while (j > 0 && json_cmp(&ms[j - 1], &m) > 0) {
			ms[j] = ms[j - 1];
			j--;
		}
		ms[j] = m;
	}
	ey_buf_char(b, '{');
	bool first = true;
	for (int64_t i = 0; i < n; i++) {
		if (i + 1 < n && json_cmp(&ms[i], &ms[i + 1]) == 0) continue;
		if (!first) ey_buf_char(b, ',');
		first = false;
		json_string(b, ms[i].key, ms[i].len);
		ey_buf_char(b, ':');
		if (ms[i].t)
			json_value(b, ms[i].t, ms[i].p, at);
		else
			json_string(b, (const char *)ms[i].p, (int64_t)strlen(ms[i].p));
	}
	ey_buf_char(b, '}');
}

static void json_items(ey_buf *b, const ey_type *t, const char *items, int64_t n, const char *at) {
	ey_buf_char(b, '[');
	for (int64_t i = 0; i < n; i++) {
		if (i) ey_buf_char(b, ',');
		json_value(b, t, items + i * (int64_t)t->size, at);
	}
	ey_buf_char(b, ']');
}

static void json_map(ey_buf *b, const ey_type *t, ey_map *m, const char *at) {
	const ey_type *kt = t->args[0], *vt = t->args[1];
	int64_t n = m->len;
	ey_buf keys = {0};
	int64_t *ends = malloc((size_t)(n ? n : 1) * sizeof *ends);
	bool strings = true;
	for (int64_t i = 0; i < n && strings; i++) {
		strings = json_raw(&keys, kt, m->keys + i * (int64_t)kt->size);
		ends[i] = keys.len;
	}
	if (strings) {
		json_member *ms = malloc((size_t)(n ? n : 1) * sizeof *ms);
		for (int64_t i = 0; i < n; i++) {
			int64_t start = i ? ends[i - 1] : 0;
			ms[i] = (json_member){keys.b + start, ends[i] - start, vt, m->vals + i * (int64_t)vt->size};
		}
		json_object(b, ms, n, at);
		free(ms);
	} else {
		ey_buf_char(b, '[');
		for (int64_t i = 0; i < n; i++) {
			if (i) ey_buf_char(b, ',');
			ey_buf_char(b, '[');
			json_value(b, kt, m->keys + i * (int64_t)kt->size, at);
			ey_buf_char(b, ',');
			json_value(b, vt, m->vals + i * (int64_t)vt->size, at);
			ey_buf_char(b, ']');
		}
		ey_buf_char(b, ']');
	}
	free(keys.b);
	free(ends);
}

/* a variant's field i's name in JSON: its own, or value, value1, … */
static const char *json_field_name(const ey_variant *v, uint32_t i, char *spare) {
	if (v->names && v->names[i] && v->names[i][0]) return v->names[i];
	if (v->n == 1) return "value";
	snprintf(spare, 16, "value%u", i + 1);
	return spare;
}

static void json_variant(ey_buf *b, const ey_type *t, const void *p, const char *at) {
	uint32_t k = ey_variant_of(t, p);
	const ey_variant *v = &t->variants[k];
	if (json_is_option(t)) {
		if (k == 0)
			json_value(b, v->fields[0], ey_variant_field(t, p, 0, 0), at);
		else
			ey_buf_str(b, "null");
		return;
	}
	if (json_is_enum(t)) {
		json_string(b, v->name, (int64_t)strlen(v->name));
		return;
	}
	json_member ms[v->n + 1];
	char spare[v->n + 1][16];
	ms[0] = (json_member){"kind", 4, NULL, v->name};
	for (uint32_t i = 0; i < v->n; i++) {
		const char *name = json_field_name(v, i, spare[i]);
		ms[i + 1] = (json_member){name, (int64_t)strlen(name), v->fields[i], ey_variant_field(t, p, k, i)};
	}
	json_object(b, ms, v->n + 1, at);
}

static void json_value(ey_buf *b, const ey_type *t, const void *p, const char *at) {
	switch (t->kind) {
	case EY_UNIT:
	case EY_NEVER:
		ey_buf_str(b, "null");
		return;
	case EY_BOOL:
		ey_buf_str(b, *(const uint8_t *)p ? "true" : "false");
		return;
	case EY_FLOAT:
		json_float(b, *(const double *)p, at);
		return;
	case EY_CHAR: {
		ey_buf r = {0};
		ey_buf_rune(&r, *(const uint32_t *)p);
		json_string(b, r.b, r.len);
		free(r.b);
		return;
	}
	case EY_BYTE:
	case EY_UINT32:
	case EY_UINT64:
		ey_buf_u64(b, ey_load_v(t, p), 0);
		return;
	case EY_INT32:
	case EY_INT64:
	case EY_SIZE:
		ey_buf_i64(b, (int64_t)ey_load_v(t, p));
		return;
	case EY_DURATION:
		ey_buf_i64(b, (int64_t)ey_load_v(t, p) / 1000000);
		return;
	case EY_INT:
		ey_int_write(b, *(const ey_v *)p);
		return;
	case EY_STRING: {
		ey_str *s = EY_STR(*(const ey_v *)p);
		json_string(b, s->b, s->len);
		return;
	}
	case EY_BYTES: {
		ey_str *s = EY_STR(*(const ey_v *)p);
		ey_buf_char(b, '[');
		for (int64_t i = 0; i < s->len; i++) {
			if (i) ey_buf_char(b, ',');
			ey_buf_u64(b, (unsigned char)s->b[i], 0);
		}
		ey_buf_char(b, ']');
		return;
	}
	case EY_LIST: {
		ey_list *l = EY_LIST(*(const ey_v *)p);
		json_items(b, t->args[0], l->items, l->len, at);
		return;
	}
	case EY_SET: {
		ey_map *m = EY_MAP(*(const ey_v *)p);
		json_items(b, t->args[0], m->keys, m->len, at);
		return;
	}
	case EY_MAP:
		json_map(b, t, EY_MAP(*(const ey_v *)p), at);
		return;
	case EY_TUPLE:
		ey_buf_char(b, '[');
		for (uint32_t i = 0; i < t->n; i++) {
			if (i) ey_buf_char(b, ',');
			json_value(b, t->args[i], ey_field(t, p, i), at);
		}
		ey_buf_char(b, ']');
		return;
	case EY_RECORD: {
		json_member ms[t->n ? t->n : 1];
		for (uint32_t i = 0; i < t->n; i++)
			ms[i] = (json_member){t->names[i], (int64_t)strlen(t->names[i]), t->args[i], ey_field(t, p, i)};
		json_object(b, ms, t->n, at);
		return;
	}
	case EY_VARIANT:
		json_variant(b, t, p, at);
		return;
	}
	/* anything else the interpreter writes as it shows: a Date, a Time, an
	 * Instant in RFC 3339, a range */
	ey_buf s = {0};
	json_raw(&s, t, p);
	json_string(b, s.b ? s.b : "", s.len);
	free(s.b);
}

ey_v ey_to_json(const char *at, const ey_type *t, const void *p) {
	ey_buf b = {0};
	json_value(&b, t, p, at);
	return ey_buf_take(&b);
}
