/* Values in general, by their type's descriptor: equality, order and
 * hashing as the interpreter has them (derived Eq, Ord and Hash), and
 * showing them exactly as the interpreter prints them (derived Show). */
#include <math.h>
#include <stdio.h>
#include <stdlib.h>

#include "eye.h"

/* ---------------------------------------------------------------- aggregates */

ey_agg *ey_agg_new(const ey_type *ty, uint32_t tag, uint32_t n) {
	if (n == 0 && ty->kind == EY_VARIANT && ty->variants[tag].unit) return ty->variants[tag].unit;
	ey_agg *a = ey_alloc(sizeof(ey_agg) + n * sizeof(ey_v), ty);
	a->tag = tag;
	a->n = n;
	return a;
}

/* the type of field i of an aggregate of type t with tag */
static const ey_type *field_type(const ey_type *t, uint32_t tag, uint32_t i) {
	switch (t->kind) {
	case EY_VARIANT:
		return t->variants[tag].fields[i];
	case EY_RANGE:
		return i < 2 ? t->args[0] : &ey_t_int64;
	default:
		return t->args[i];
	}
}

void ey_agg_own(ey_v *p) {
	ey_agg *a = EY_AGG(*p);
	if (a->h.rc == 1) return;
	ey_agg *b = ey_agg_new(a->h.ty, a->tag, a->n);
	memcpy(b->f, a->f, a->n * sizeof(ey_v));
	for (uint32_t i = 0; i < a->n; i++)
		if (ey_counted(field_type(a->h.ty, a->tag, i))) ey_dup(b->f[i]);
	ey_drop(*p);
	*p = EY_V(b);
}

static ey_v variant1(const ey_type *t, uint32_t tag, ey_v v) {
	ey_agg *a = ey_agg_new(t, tag, 1);
	a->f[0] = v;
	return EY_V(a);
}

ey_v ey_some(const ey_type *opt, ey_v v) { return variant1(opt, 0, v); }
ey_v ey_none(const ey_type *opt) { return EY_V(ey_agg_new(opt, 1, 0)); }
ey_v ey_ok(const ey_type *res, ey_v v) { return variant1(res, 0, v); }
ey_v ey_err(const ey_type *res, ey_v v) { return variant1(res, 1, v); }

ey_v ey_range(const ey_type *ty, ey_v start, ey_v end, uint32_t flags) {
	ey_agg *a = ey_agg_new(ty, 0, 3);
	a->f[0] = start;
	a->f[1] = end;
	a->f[2] = flags;
	return EY_V(a);
}

ey_fn *ey_fn_new(const ey_type *ty, ey_code code, uint32_t n) {
	ey_fn *f = ey_alloc(sizeof(ey_fn) + n * sizeof(ey_v), ty);
	f->code = code;
	f->n = n;
	return f;
}

/* ---------------------------------------------------------------- equality */

static bool float_eq(double a, double b) {
	if (isnan(a) || isnan(b)) return isnan(a) && isnan(b);
	return a == b && signbit(a) == signbit(b);
}

bool ey_eq(const ey_type *t, ey_v a, ey_v b) {
	switch (t->kind) {
	case EY_UNIT:
	case EY_NEVER:
		return true;
	case EY_FLOAT:
		return float_eq(ey_v2f(a), ey_v2f(b));
	case EY_INT:
		return ey_int_eq(a, b);
	case EY_STRING:
	case EY_BYTES:
		return ey_str_eq(a, b);
	case EY_INSTANT:
		return ((ey_instant *)(uintptr_t)a)->ns == ((ey_instant *)(uintptr_t)b)->ns;
	case EY_LIST: {
		ey_list *x = EY_LIST(a), *y = EY_LIST(b);
		if (x->len != y->len) return false;
		for (int64_t i = 0; i < x->len; i++)
			if (!ey_eq(t->args[0], x->items[i], y->items[i])) return false;
		return true;
	}
	case EY_SET:
	case EY_MAP: {
		ey_map *x = EY_MAP(a), *y = EY_MAP(b);
		if (x->len != y->len) return false;
		for (int64_t i = 0; i < x->len; i++) {
			int64_t j = ey_map_find(y, x->keys[i]);
			if (j < 0) return false;
			if (t->kind == EY_MAP && !ey_eq(t->args[1], x->vals[i], y->vals[j])) return false;
		}
		return true;
	}
	case EY_TUPLE:
	case EY_RECORD:
	case EY_VARIANT:
	case EY_RANGE: {
		ey_agg *x = EY_AGG(a), *y = EY_AGG(b);
		if (x->tag != y->tag || x->n != y->n) return false;
		for (uint32_t i = 0; i < x->n; i++)
			if (!ey_eq(field_type(t, x->tag, i), x->f[i], y->f[i])) return false;
		return true;
	}
	case EY_DYN: {
		/* a dyn value's own descriptor says what is inside */
		ey_dyn *x = EY_DYNP(a), *y = EY_DYNP(b);
		return x->h.ty == y->h.ty && ey_eq(x->h.ty->args[0], x->val, y->val);
	}
	default:
		return a == b;
	}
}

/* ---------------------------------------------------------------- order */

#define CMP(x, y) ((x) < (y) ? -1 : (x) > (y))

int ey_cmp(const ey_type *t, ey_v a, ey_v b) {
	switch (t->kind) {
	case EY_INT:
		return ey_int_cmp(a, b);
	case EY_FLOAT:
		return CMP(ey_v2f(a), ey_v2f(b));
	case EY_STRING:
	case EY_BYTES:
		return ey_str_cmp(a, b);
	case EY_CHAR:
	case EY_INT32:
	case EY_INT64:
	case EY_DURATION:
	case EY_SIZE:
	case EY_DATE:
	case EY_TIME:
		return CMP((int64_t)a, (int64_t)b);
	case EY_BOOL:
	case EY_BYTE:
	case EY_UINT32:
	case EY_UINT64:
		return CMP(a, b);
	case EY_INSTANT:
		return CMP(((ey_instant *)(uintptr_t)a)->ns, ((ey_instant *)(uintptr_t)b)->ns);
	case EY_LIST: {
		ey_list *x = EY_LIST(a), *y = EY_LIST(b);
		for (int64_t i = 0; i < x->len && i < y->len; i++) {
			int c = ey_cmp(t->args[0], x->items[i], y->items[i]);
			if (c) return c;
		}
		return CMP(x->len, y->len);
	}
	case EY_TUPLE:
	case EY_RECORD:
	case EY_VARIANT: {
		ey_agg *x = EY_AGG(a), *y = EY_AGG(b);
		if (x->tag != y->tag) return CMP(x->tag, y->tag);
		for (uint32_t i = 0; i < x->n && i < y->n; i++) {
			int c = ey_cmp(field_type(t, x->tag, i), x->f[i], y->f[i]);
			if (c) return c;
		}
		return 0;
	}
	default:
		return 0;
	}
}

/* ---------------------------------------------------------------- hashing */

static uint64_t fin(uint64_t x) {
	x ^= x >> 33;
	x *= 0xff51afd7ed558ccdULL;
	x ^= x >> 33;
	x *= 0xc4ceb9fe1a85ec53ULL;
	x ^= x >> 33;
	return x;
}

static uint64_t mix(uint64_t h, uint64_t x) { return fin(h ^ (x + 0x9e3779b97f4a7c15ULL + (h << 6) + (h >> 2))); }

uint64_t ey_hash(const ey_type *t, ey_v v) {
	switch (t->kind) {
	case EY_UNIT:
		return 0;
	case EY_FLOAT: {
		double d = ey_v2f(v);
		return isnan(d) ? 0x7ff8 : fin(v);
	}
	case EY_INT: {
		if (EY_IS_SMALL(v)) return fin(v);
		ey_big *g = (ey_big *)(uintptr_t)v;
		uint64_t h = (uint64_t)g->neg;
		for (uint32_t i = 0; i < g->n; i++) h = mix(h, g->d[i]);
		return h;
	}
	case EY_STRING:
	case EY_BYTES: {
		ey_str *s = EY_STR(v);
		uint64_t h = 1469598103934665603ULL;
		for (int64_t i = 0; i < s->len; i++) h = (h ^ (unsigned char)s->b[i]) * 1099511628211ULL;
		return fin(h);
	}
	case EY_INSTANT:
		return fin((uint64_t)((ey_instant *)(uintptr_t)v)->ns);
	case EY_LIST: {
		ey_list *l = EY_LIST(v);
		uint64_t h = 7;
		for (int64_t i = 0; i < l->len; i++) h = mix(h, ey_hash(t->args[0], l->items[i]));
		return h;
	}
	case EY_SET:
	case EY_MAP: {
		/* the same whatever the order */
		ey_map *m = EY_MAP(v);
		uint64_t h = 11;
		for (int64_t i = 0; i < m->len; i++) {
			uint64_t e = ey_hash(t->args[0], m->keys[i]);
			if (t->kind == EY_MAP) e = mix(e, ey_hash(t->args[1], m->vals[i]));
			h += fin(e);
		}
		return h;
	}
	case EY_TUPLE:
	case EY_RECORD:
	case EY_VARIANT:
	case EY_RANGE: {
		ey_agg *a = EY_AGG(v);
		uint64_t h = mix(13, a->tag);
		for (uint32_t i = 0; i < a->n; i++) h = mix(h, ey_hash(field_type(t, a->tag, i), a->f[i]));
		return h;
	}
	case EY_DYN:
		return ey_hash(EY_DYNP(v)->h.ty->args[0], EY_DYNP(v)->val);
	default:
		return fin(v);
	}
}

/* ---------------------------------------------------------------- showing */

/* ey_float_write writes d as Go's strconv.FormatFloat(d, 'g', -1, 64) does:
 * the shortest digits that read back as d, in %e form when the exponent
 * is below -4 or at least 6. */
void ey_float_write(ey_buf *b, double d) {
	if (isnan(d)) {
		ey_buf_str(b, "NaN");
		return;
	}
	if (isinf(d)) {
		ey_buf_str(b, d > 0 ? "+Inf" : "-Inf");
		return;
	}
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
	if (exp < -4 || exp >= 6) {
		ey_buf_char(b, digits[0]);
		if (nd > 1) {
			ey_buf_char(b, '.');
			ey_buf_add(b, digits + 1, nd - 1);
		}
		ey_buf_printf(b, "e%c%02d", exp < 0 ? '-' : '+', exp < 0 ? -exp : exp);
		return;
	}
	int dp = exp + 1;
	if (dp > 0) {
		for (int i = 0; i < dp; i++) ey_buf_char(b, i < nd ? digits[i] : '0');
	} else {
		ey_buf_char(b, '0');
	}
	int frac = nd - dp;
	if (frac > 0) {
		ey_buf_char(b, '.');
		for (int i = 0; i < frac; i++) {
			int j = dp + i;
			ey_buf_char(b, j >= 0 && j < nd ? digits[j] : '0');
		}
	}
}

/* whether Go's strconv.Quote writes r as it is */
static bool printable(uint32_t r) {
	if (r < 0x20 || r == 0x7f) return false;
	if (r >= 0x80 && r <= 0x9f) return false;
	if (r == 0xad || r == 0x2028 || r == 0x2029 || r == 0xfeff) return false;
	if (r >= 0xfff9 && r <= 0xfffb) return false;
	if ((r & 0xfffe) == 0xfffe) return false;
	return true;
}

static void quote_rune(ey_buf *b, uint32_t r, char q) {
	switch (r) {
	case '\a': ey_buf_str(b, "\\a"); return;
	case '\b': ey_buf_str(b, "\\b"); return;
	case '\f': ey_buf_str(b, "\\f"); return;
	case '\n': ey_buf_str(b, "\\n"); return;
	case '\r': ey_buf_str(b, "\\r"); return;
	case '\t': ey_buf_str(b, "\\t"); return;
	case '\v': ey_buf_str(b, "\\v"); return;
	case '\\': ey_buf_str(b, "\\\\"); return;
	}
	if (r == (uint32_t)q) {
		ey_buf_char(b, '\\');
		ey_buf_char(b, q);
		return;
	}
	if (printable(r)) {
		ey_buf_rune(b, r);
	} else if (r < 0x80) {
		ey_buf_printf(b, "\\x%02x", r);
	} else if (r < 0x10000) {
		ey_buf_printf(b, "\\u%04x", r);
	} else {
		ey_buf_printf(b, "\\U%08x", r);
	}
}

static void quote(ey_buf *b, ey_str *s) {
	ey_buf_char(b, '"');
	for (int64_t i = 0; i < s->len;) {
		uint32_t r;
		int64_t k = ey_utf8_decode(s->b + i, s->len - i, &r);
		if (r == 0xfffd && k == 1 && (unsigned char)s->b[i] >= 0x80)
			ey_buf_printf(b, "\\x%02x", (unsigned char)s->b[i]);
		else
			quote_rune(b, r, '"');
		i += k;
	}
	ey_buf_char(b, '"');
}

static void show_duration(ey_buf *b, int64_t d) {
	static const struct {
		const char *name;
		int64_t ns;
	} units[] = {{"days", 86400000000000LL}, {"h", 3600000000000LL}, {"min", 60000000000LL}, {"s", 1000000000LL}, {"ms", 1000000LL}};
	for (int i = 0; i < 5; i++) {
		if (d != 0 && d % units[i].ns == 0) {
			int64_t n = d / units[i].ns;
			if (i == 0 && n == 1) {
				ey_buf_str(b, "1 day");
				return;
			}
			ey_buf_i64(b, n);
			ey_buf_char(b, ' ');
			ey_buf_str(b, units[i].name);
			return;
		}
	}
	ey_buf_i64(b, d / 1000000);
	ey_buf_str(b, " ms");
}

static void show_size(ey_buf *b, int64_t s) {
	static const struct {
		const char *name;
		int64_t n;
	} units[] = {{"GiB", 1LL << 30}, {"MiB", 1LL << 20}, {"KiB", 1LL << 10}};
	for (int i = 0; i < 3; i++) {
		if (s != 0 && s % units[i].n == 0) {
			ey_buf_i64(b, s / units[i].n);
			ey_buf_char(b, ' ');
			ey_buf_str(b, units[i].name);
			return;
		}
	}
	ey_buf_i64(b, s);
	ey_buf_str(b, " B");
}

void ey_instant_write(ey_buf *b, ey_v v); /* time.c */

static void show_list(ey_buf *b, const ey_type *t, const ey_v *items, int64_t n) {
	for (int64_t i = 0; i < n; i++) {
		if (i > 0) ey_buf_str(b, ", ");
		ey_show_to(b, t, items[i], true);
	}
}

void ey_show_to(ey_buf *b, const ey_type *t, ey_v v, bool nested) {
	switch (t->kind) {
	case EY_UNIT:
		ey_buf_str(b, "()");
		return;
	case EY_BOOL:
		ey_buf_str(b, v ? "true" : "false");
		return;
	case EY_FLOAT: {
		int64_t start = b->len;
		ey_float_write(b, ey_v2f(v));
		bool plain = true;
		for (int64_t i = start; i < b->len; i++)
			if (strchr(".eEn", b->b[i])) plain = false;
		if (plain) ey_buf_str(b, ".0");
		return;
	}
	case EY_CHAR:
		if (nested) {
			ey_buf_char(b, '\'');
			quote_rune(b, (uint32_t)v, '\'');
			ey_buf_char(b, '\'');
		} else {
			ey_buf_rune(b, (uint32_t)v);
		}
		return;
	case EY_BYTE:
	case EY_UINT32:
	case EY_UINT64:
		ey_buf_u64(b, v, 0);
		return;
	case EY_INT32:
	case EY_INT64:
		ey_buf_i64(b, (int64_t)v);
		return;
	case EY_DURATION:
		show_duration(b, (int64_t)v);
		return;
	case EY_SIZE:
		show_size(b, (int64_t)v);
		return;
	case EY_DATE: {
		int64_t y, m, d;
		ey_civil_from_days((int64_t)v, &y, &m, &d);
		ey_buf_printf(b, "%04lld-%02lld-%02lld", (long long)y, (long long)m, (long long)d);
		return;
	}
	case EY_TIME: {
		int64_t s = (int64_t)v;
		ey_buf_printf(b, "%02lld:%02lld", (long long)(s / 3600), (long long)(s / 60 % 60));
		if (s % 60) ey_buf_printf(b, ":%02lld", (long long)(s % 60));
		return;
	}
	case EY_INT:
		ey_int_write(b, v);
		return;
	case EY_STRING:
		if (nested)
			quote(b, EY_STR(v));
		else
			ey_buf_add(b, EY_STR(v)->b, EY_STR(v)->len);
		return;
	case EY_BYTES: {
		ey_str *s = EY_STR(v);
		ey_buf_char(b, '[');
		for (int64_t i = 0; i < s->len; i++) {
			if (i) ey_buf_str(b, ", ");
			ey_buf_u64(b, (unsigned char)s->b[i], 0);
		}
		ey_buf_char(b, ']');
		return;
	}
	case EY_INSTANT:
		ey_instant_write(b, v);
		return;
	case EY_LIST:
		ey_buf_char(b, '[');
		show_list(b, t->args[0], EY_LIST(v)->items, EY_LIST(v)->len);
		ey_buf_char(b, ']');
		return;
	case EY_SET: {
		ey_map *m = EY_MAP(v);
		if (m->len == 0) {
			ey_buf_str(b, "{}");
			return;
		}
		ey_buf_char(b, '{');
		show_list(b, t->args[0], m->keys, m->len);
		ey_buf_char(b, '}');
		return;
	}
	case EY_MAP: {
		ey_map *m = EY_MAP(v);
		if (m->len == 0) {
			ey_buf_str(b, "{}");
			return;
		}
		ey_buf_str(b, "{ ");
		for (int64_t i = 0; i < m->len; i++) {
			if (i > 0) ey_buf_str(b, ", ");
			ey_show_to(b, t->args[0], m->keys[i], true);
			ey_buf_str(b, ": ");
			ey_show_to(b, t->args[1], m->vals[i], true);
		}
		ey_buf_str(b, " }");
		return;
	}
	case EY_TUPLE: {
		ey_agg *a = EY_AGG(v);
		ey_buf_char(b, '(');
		for (uint32_t i = 0; i < a->n; i++) {
			if (i > 0) ey_buf_str(b, ", ");
			ey_show_to(b, t->args[i], a->f[i], true);
		}
		ey_buf_char(b, ')');
		return;
	}
	case EY_RECORD: {
		ey_agg *a = EY_AGG(v);
		ey_buf_str(b, "{ ");
		for (uint32_t i = 0; i < a->n; i++) {
			if (i > 0) ey_buf_str(b, ", ");
			ey_buf_str(b, t->names[i]);
			ey_buf_str(b, ": ");
			ey_show_to(b, t->args[i], a->f[i], true);
		}
		ey_buf_str(b, " }");
		return;
	}
	case EY_VARIANT: {
		ey_agg *a = EY_AGG(v);
		const ey_variant *vr = &t->variants[a->tag];
		ey_buf_str(b, vr->name);
		if (a->n == 0) return;
		ey_buf_char(b, '(');
		for (uint32_t i = 0; i < a->n; i++) {
			if (i > 0) ey_buf_str(b, ", ");
			if (vr->names[i][0]) {
				ey_buf_str(b, vr->names[i]);
				ey_buf_str(b, ": ");
			}
			ey_show_to(b, vr->fields[i], a->f[i], true);
		}
		ey_buf_char(b, ')');
		return;
	}
	case EY_FN:
		ey_buf_str(b, "fn ");
		ey_buf_str(b, t->name ? t->name : "lambda");
		return;
	case EY_DYN:
		ey_show_to(b, EY_DYNP(v)->h.ty->args[0], EY_DYNP(v)->val, nested);
		return;
	case EY_RANGE: {
		ey_agg *a = EY_AGG(v);
		if (!(a->f[2] & EY_RANGE_NO_START)) ey_show_to(b, t->args[0], a->f[0], true);
		ey_buf_str(b, a->f[2] & EY_RANGE_INCLUSIVE ? "..=" : "..");
		if (!(a->f[2] & EY_RANGE_NO_END)) ey_show_to(b, t->args[0], a->f[1], true);
		return;
	}
	default:
		ey_buf_str(b, "?");
	}
}

ey_v ey_show_int(ey_v v) {
	if (EY_IS_SMALL(v)) {
		char s[24];
		char *p = ey_fmt_i64(s + sizeof s, EY_SMALL_VAL(v));
		return ey_str_new(p, s + sizeof s - p, &ey_t_string);
	}
	ey_buf b = {0};
	ey_int_write(&b, v);
	return ey_buf_take(&b);
}

ey_v ey_show(const ey_type *t, ey_v v) {
	switch (t->kind) {
	case EY_INT:
		return ey_show_int(v);
	case EY_INT32:
	case EY_INT64: {
		char s[24];
		char *p = ey_fmt_i64(s + sizeof s, (int64_t)v);
		return ey_str_new(p, s + sizeof s - p, &ey_t_string);
	}
	case EY_BYTE:
	case EY_UINT32:
	case EY_UINT64: {
		char s[24];
		char *p = ey_fmt_u64(s + sizeof s, v);
		return ey_str_new(p, s + sizeof s - p, &ey_t_string);
	}
	case EY_STRING:
		ey_dup(v); /* a String shows as itself */
		return v;
	default:
		break;
	}
	ey_buf b = {0};
	ey_show_to(&b, t, v, false);
	return ey_buf_take(&b);
}
