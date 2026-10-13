/* Values in general, by their type's descriptor, which says how they are
 * laid out (eye.h): records, tuples and variants made and taken apart;
 * equality, order and hashing as the interpreter has them (derived Eq, Ord
 * and Hash); and showing values exactly as the interpreter prints them
 * (derived Show). */
#include <math.h>
#include <stdio.h>
#include <stdlib.h>

#include "eye.h"

/* ---------------------------------------------------------------- records, tuples and variants */

static uint64_t load_u(const void *p, uint32_t size) {
	switch (size) {
	case 1:
		return *(const uint8_t *)p;
	case 2:
		return *(const uint16_t *)p;
	case 4:
		return *(const uint32_t *)p;
	default:
		return *(const uint64_t *)p;
	}
}

static void store_u(void *p, uint32_t size, uint64_t v) {
	switch (size) {
	case 1:
		*(uint8_t *)p = (uint8_t)v;
		break;
	case 2:
		*(uint16_t *)p = (uint16_t)v;
		break;
	case 4:
		*(uint32_t *)p = (uint32_t)v;
		break;
	default:
		*(uint64_t *)p = v;
		break;
	}
}

uint32_t ey_variant_of(const ey_type *t, const void *p) {
	if (t->repr == EY_R_ENUM) return (uint32_t)load_u(p, t->size);
	switch (t->tag) {
	case EY_TAG_FIELD:
		return (uint32_t)load_u((const char *)p + t->tagoff, t->tagsize);
	case EY_TAG_NICHE: {
		uint64_t v = load_u((const char *)p + t->tagoff, t->tagsize);
		uint32_t dataful = 0;
		for (uint32_t i = 0; i < t->n; i++) {
			if (t->variants[i].n == 0) {
				if (t->variants[i].value == v) return i;
			} else {
				dataful = i;
			}
		}
		return dataful;
	}
	case EY_TAG_BOX: {
		ey_v w = *(const ey_v *)p;
		if (!ey_is_obj(w)) {
			for (uint32_t i = 0; i < t->n; i++)
				if (t->variants[i].n == 0 && t->variants[i].value == w) return i;
			return 0;
		}
		return EY_OBJ(w)->tag;
	}
	}
	return 0;
}

void *ey_variant_field(const ey_type *t, const void *p, uint32_t tag, uint32_t i) {
	const ey_variant *v = &t->variants[tag];
	if (t->tag == EY_TAG_BOX) return (char *)(uintptr_t)(*(const ey_v *)p) + v->offs[i];
	return (char *)p + v->offs[i];
}

void ey_variant_make(const ey_type *t, void *out, uint32_t tag, const void *const *fields) {
	const ey_variant *v = &t->variants[tag];
	if (t->repr == EY_R_ENUM) {
		store_u(out, t->size, v->value);
		return;
	}
	char *base = out;
	if (t->tag == EY_TAG_NICHE) {
		/* Option(Int), Option(String), Option(List(a)): the payload or
		 * the niche fills the whole value */
		if (v->n == 1 && v->fields[0]->size == t->size) {
			if (t->size == 8)
				memcpy(out, fields[0], 8);
			else
				memcpy(out, fields[0], t->size);
			return;
		}
		if (v->n == 0 && t->tagsize == t->size) {
			store_u(out, t->tagsize, v->value);
			return;
		}
	}
	if (t->tag == EY_TAG_BOX) {
		if (v->n == 0) {
			*(ey_v *)out = v->value;
			return;
		}
		ey_obj *o = ey_alloc(v->objsize, t);
		o->tag = tag;
		*(ey_v *)out = EY_V(o);
		base = (char *)o;
	} else {
		memset(out, 0, t->size);
		if (t->tag == EY_TAG_FIELD || (t->tag == EY_TAG_NICHE && v->n == 0))
			store_u(base + t->tagoff, t->tagsize, v->value);
	}
	for (uint32_t i = 0; i < v->n; i++) memcpy(base + v->offs[i], fields[i], v->fields[i]->size);
}

void *ey_field(const ey_type *t, const void *p, uint32_t i) {
	if (t->boxed) return (char *)(uintptr_t)(*(const ey_v *)p) + t->offs[i];
	return (char *)p + t->offs[i];
}

void ey_tuple_make(const ey_type *t, void *out, const void *const *fields) {
	char *base = out;
	if (t->boxed) {
		ey_obj *o = ey_alloc(t->objsize, t);
		*(ey_v *)out = EY_V(o);
		base = (char *)o;
	} else {
		memset(out, 0, t->size);
	}
	for (uint32_t i = 0; i < t->n; i++) memcpy(base + t->offs[i], fields[i], t->args[i]->size);
}

void ey_box_own(ey_v *p) {
	ey_obj *o = EY_OBJ(*p);
	if (o->rc == 1) return;
	const ey_type *t = o->ty;
	size_t size;
	uint32_t n;
	const ey_type *const *ts;
	const uint32_t *offs;
	if (t->kind == EY_VARIANT) {
		const ey_variant *v = &t->variants[o->tag];
		size = v->objsize, n = v->n, ts = v->fields, offs = v->offs;
	} else {
		size = t->objsize, n = t->n, ts = t->args, offs = t->offs;
	}
	ey_obj *q = ey_alloc(size, t);
	q->tag = o->tag;
	memcpy(q + 1, o + 1, size - sizeof(ey_obj));
	for (uint32_t i = 0; i < n; i++) ey_dup_at(ts[i], (char *)q + offs[i]);
	ey_drop(*p);
	*p = EY_V(q);
}

void ey_ok(const ey_type *res, void *out, const void *v) { ey_variant_make(res, out, 0, (const void *const[]){v}); }
void ey_err(const ey_type *res, void *out, const void *v) { ey_variant_make(res, out, 1, (const void *const[]){v}); }

static void make_v(const ey_type *t, void *out, uint32_t tag, ey_v w) {
	uint64_t tmp = 0;
	ey_store_v(t->variants[tag].fields[0], &tmp, w);
	ey_variant_make(t, out, tag, (const void *const[]){&tmp});
}

void ey_some_v(const ey_type *opt, void *out, ey_v v) { make_v(opt, out, 0, v); }
void ey_ok_v(const ey_type *res, void *out, ey_v v) { make_v(res, out, 0, v); }
void ey_err_v(const ey_type *res, void *out, ey_v v) { make_v(res, out, 1, v); }

ey_v ey_range(const ey_type *ty, ey_v start, ey_v end, uint32_t flags) {
	ey_range_obj *r = ey_alloc(sizeof(ey_range_obj), ty);
	r->start = start;
	r->end = end;
	r->flags = flags;
	return EY_V(r);
}

ey_fn *ey_fn_new(const ey_type *ty, ey_code code) {
	ey_fn *f = ey_alloc(ty->objsize, ty);
	f->code = code;
	return f;
}

/* ---------------------------------------------------------------- equality */

static bool float_eq(double a, double b) {
	if (isnan(a) || isnan(b)) return isnan(a) && isnan(b);
	return a == b && signbit(a) == signbit(b);
}

#define WORDOF(p) (*(const ey_v *)(p))
#define DBL(p) (*(const double *)(p))

bool ey_eq(const ey_type *t, const void *a, const void *b) {
	switch (t->kind) {
	case EY_UNIT:
	case EY_NEVER:
		return true;
	case EY_FLOAT:
		return float_eq(DBL(a), DBL(b));
	case EY_INT:
		return ey_int_eq(WORDOF(a), WORDOF(b));
	case EY_STRING:
	case EY_BYTES:
		return ey_str_eq(WORDOF(a), WORDOF(b));
	case EY_INSTANT:
		return ((ey_instant *)(uintptr_t)WORDOF(a))->ns == ((ey_instant *)(uintptr_t)WORDOF(b))->ns;
	case EY_LIST: {
		ey_list *x = EY_LIST(WORDOF(a)), *y = EY_LIST(WORDOF(b));
		if (x->len != y->len) return false;
		for (int64_t i = 0; i < x->len; i++)
			if (!ey_eq(t->args[0], ey_list_item(x, i), ey_list_item(y, i))) return false;
		return true;
	}
	case EY_SET:
	case EY_MAP: {
		ey_map *x = EY_MAP(WORDOF(a)), *y = EY_MAP(WORDOF(b));
		if (x->len != y->len) return false;
		uint32_t ks = t->args[0]->size;
		for (int64_t i = 0; i < x->len; i++) {
			int64_t j = ey_map_find(y, x->keys + i * ks);
			if (j < 0) return false;
			if (t->kind == EY_MAP) {
				uint32_t vs = t->args[1]->size;
				if (!ey_eq(t->args[1], x->vals + i * vs, y->vals + j * vs)) return false;
			}
		}
		return true;
	}
	case EY_RANGE: {
		ey_range_obj *x = EY_RANGEP(WORDOF(a)), *y = EY_RANGEP(WORDOF(b));
		return x->flags == y->flags && ey_eq(t->args[0], &x->start, &y->start) && ey_eq(t->args[0], &x->end, &y->end);
	}
	case EY_TUPLE:
	case EY_RECORD:
		for (uint32_t i = 0; i < t->n; i++)
			if (!ey_eq(t->args[i], ey_field(t, a, i), ey_field(t, b, i))) return false;
		return true;
	case EY_VARIANT: {
		uint32_t x = ey_variant_of(t, a), y = ey_variant_of(t, b);
		if (x != y) return false;
		const ey_variant *v = &t->variants[x];
		for (uint32_t i = 0; i < v->n; i++)
			if (!ey_eq(v->fields[i], ey_variant_field(t, a, x, i), ey_variant_field(t, b, x, i))) return false;
		return true;
	}
	case EY_DYN: {
		/* a dyn value's own descriptor says what is inside */
		ey_dyn *x = EY_DYNP(WORDOF(a)), *y = EY_DYNP(WORDOF(b));
		return x->h.ty == y->h.ty && ey_eq(x->h.ty->args[0], EY_DYN_VAL(x), EY_DYN_VAL(y));
	}
	default:
		if (t->repr == EY_R_WORD) return WORDOF(a) == WORDOF(b);
		return ey_load_v(t, a) == ey_load_v(t, b);
	}
}

/* ---------------------------------------------------------------- order */

#define CMP(x, y) ((x) < (y) ? -1 : (x) > (y))

int ey_cmp(const ey_type *t, const void *a, const void *b) {
	switch (t->kind) {
	case EY_INT:
		return ey_int_cmp(WORDOF(a), WORDOF(b));
	case EY_FLOAT:
		return CMP(DBL(a), DBL(b));
	case EY_STRING:
	case EY_BYTES:
		return ey_str_cmp(WORDOF(a), WORDOF(b));
	case EY_INT32:
	case EY_INT64:
	case EY_DURATION:
	case EY_SIZE:
	case EY_DATE:
	case EY_TIME:
		return CMP((int64_t)ey_load_v(t, a), (int64_t)ey_load_v(t, b));
	case EY_BOOL:
	case EY_BYTE:
	case EY_CHAR:
	case EY_UINT32:
	case EY_UINT64:
		return CMP(ey_load_v(t, a), ey_load_v(t, b));
	case EY_INSTANT:
		return CMP(((ey_instant *)(uintptr_t)WORDOF(a))->ns, ((ey_instant *)(uintptr_t)WORDOF(b))->ns);
	case EY_LIST: {
		ey_list *x = EY_LIST(WORDOF(a)), *y = EY_LIST(WORDOF(b));
		for (int64_t i = 0; i < x->len && i < y->len; i++) {
			int c = ey_cmp(t->args[0], ey_list_item(x, i), ey_list_item(y, i));
			if (c) return c;
		}
		return CMP(x->len, y->len);
	}
	case EY_TUPLE:
	case EY_RECORD:
		for (uint32_t i = 0; i < t->n; i++) {
			int c = ey_cmp(t->args[i], ey_field(t, a, i), ey_field(t, b, i));
			if (c) return c;
		}
		return 0;
	case EY_VARIANT: {
		uint32_t x = ey_variant_of(t, a), y = ey_variant_of(t, b);
		if (x != y) return CMP(x, y);
		const ey_variant *v = &t->variants[x];
		for (uint32_t i = 0; i < v->n; i++) {
			int c = ey_cmp(v->fields[i], ey_variant_field(t, a, x, i), ey_variant_field(t, b, x, i));
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

/* ey_str_hash reads a string's bytes eight at a time; maps call it for
 * every probe, so it must be cheap for short keys. */
uint64_t ey_str_hash(ey_v v) {
	ey_str *s = EY_STR(v);
	const unsigned char *p = (const unsigned char *)s->b;
	int64_t n = s->len;
	uint64_t h = 0x9e3779b97f4a7c15ULL ^ (uint64_t)n;
	for (; n >= 8; p += 8, n -= 8) {
		uint64_t w;
		memcpy(&w, p, 8);
		h = (h ^ w) * 0xff51afd7ed558ccdULL;
		h ^= h >> 29;
	}
	uint64_t w = 0;
	if (n > 0 && ((uintptr_t)p & 7) == 0) {
		/* the last bytes in one aligned load, which can't cross a page
		 * the string doesn't reach; the bytes past its end are masked */
		memcpy(&w, p, 8);
		w &= ~UINT64_C(0) >> (64 - 8 * n);
	} else {
		for (int64_t i = 0; i < n; i++) w |= (uint64_t)p[i] << (8 * i);
	}
	return fin(h ^ w);
}

uint64_t ey_hash(const ey_type *t, const void *p) {
	switch (t->kind) {
	case EY_UNIT:
	case EY_NEVER:
		return 0;
	case EY_FLOAT: {
		double d = DBL(p);
		return isnan(d) ? 0x7ff8 : fin(ey_f2v(d));
	}
	case EY_INT: {
		ey_v v = WORDOF(p);
		if (EY_IS_SMALL(v)) return fin(v);
		ey_big *g = (ey_big *)(uintptr_t)v;
		uint64_t h = (uint64_t)g->neg;
		for (uint32_t i = 0; i < g->n; i++) h = mix(h, g->d[i]);
		return h;
	}
	case EY_STRING:
	case EY_BYTES:
		return ey_str_hash(WORDOF(p));
	case EY_INSTANT:
		return fin((uint64_t)((ey_instant *)(uintptr_t)WORDOF(p))->ns);
	case EY_LIST: {
		ey_list *l = EY_LIST(WORDOF(p));
		uint64_t h = 7;
		for (int64_t i = 0; i < l->len; i++) h = mix(h, ey_hash(t->args[0], ey_list_item(l, i)));
		return h;
	}
	case EY_SET:
	case EY_MAP: {
		/* the same whatever the order */
		ey_map *m = EY_MAP(WORDOF(p));
		uint64_t h = 11;
		uint32_t ks = t->args[0]->size;
		for (int64_t i = 0; i < m->len; i++) {
			uint64_t e = ey_hash(t->args[0], m->keys + i * ks);
			if (t->kind == EY_MAP) e = mix(e, ey_hash(t->args[1], m->vals + i * t->args[1]->size));
			h += fin(e);
		}
		return h;
	}
	case EY_RANGE: {
		ey_range_obj *r = EY_RANGEP(WORDOF(p));
		return mix(mix(mix(17, r->flags), ey_hash(t->args[0], &r->start)), ey_hash(t->args[0], &r->end));
	}
	case EY_TUPLE:
	case EY_RECORD: {
		uint64_t h = 13;
		for (uint32_t i = 0; i < t->n; i++) h = mix(h, ey_hash(t->args[i], ey_field(t, p, i)));
		return h;
	}
	case EY_VARIANT: {
		uint32_t x = ey_variant_of(t, p);
		const ey_variant *v = &t->variants[x];
		uint64_t h = mix(13, x);
		for (uint32_t i = 0; i < v->n; i++) h = mix(h, ey_hash(v->fields[i], ey_variant_field(t, p, x, i)));
		return h;
	}
	case EY_DYN: {
		ey_dyn *d = EY_DYNP(WORDOF(p));
		return ey_hash(d->h.ty->args[0], EY_DYN_VAL(d));
	}
	default:
		if (t->repr == EY_R_WORD) return fin(WORDOF(p));
		return fin(ey_load_v(t, p));
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

static void show_items(ey_buf *b, const ey_type *t, const char *items, int64_t n) {
	for (int64_t i = 0; i < n; i++) {
		if (i > 0) ey_buf_str(b, ", ");
		ey_show_to(b, t, items + i * (int64_t)t->size, true);
	}
}

/* above zero while join writes elements: the interpreter's join writes
 * them in the derived form, without their impls of Show */
static int show_plain;

void ey_show_plain_to(ey_buf *b, const ey_type *t, const void *p, bool nested) {
	show_plain++;
	ey_show_to(b, t, p, nested);
	show_plain--;
}

void ey_show_to(ey_buf *b, const ey_type *t, const void *p, bool nested) {
	if (t->show && !show_plain) {
		ey_v s = t->show(p);
		ey_buf_add(b, EY_STR(s)->b, EY_STR(s)->len);
		ey_drop(s);
		return;
	}
	switch (t->kind) {
	case EY_UNIT:
		ey_buf_str(b, "()");
		return;
	case EY_BOOL:
		ey_buf_str(b, *(const uint8_t *)p ? "true" : "false");
		return;
	case EY_FLOAT: {
		int64_t start = b->len;
		ey_float_write(b, DBL(p));
		bool plain = true;
		for (int64_t i = start; i < b->len; i++)
			if (strchr(".eEn", b->b[i])) plain = false;
		if (plain) ey_buf_str(b, ".0");
		return;
	}
	case EY_CHAR: {
		uint32_t r = *(const uint32_t *)p;
		if (nested) {
			ey_buf_char(b, '\'');
			quote_rune(b, r, '\'');
			ey_buf_char(b, '\'');
		} else {
			ey_buf_rune(b, r);
		}
		return;
	}
	case EY_BYTE:
	case EY_UINT32:
	case EY_UINT64:
		ey_buf_u64(b, ey_load_v(t, p), 0);
		return;
	case EY_INT32:
	case EY_INT64:
		ey_buf_i64(b, (int64_t)ey_load_v(t, p));
		return;
	case EY_DURATION:
		show_duration(b, (int64_t)ey_load_v(t, p));
		return;
	case EY_SIZE:
		show_size(b, (int64_t)ey_load_v(t, p));
		return;
	case EY_DATE: {
		int64_t y, m, d;
		ey_civil_from_days((int64_t)ey_load_v(t, p), &y, &m, &d);
		ey_buf_printf(b, "%04lld-%02lld-%02lld", (long long)y, (long long)m, (long long)d);
		return;
	}
	case EY_TIME: {
		int64_t s = (int64_t)ey_load_v(t, p);
		ey_buf_printf(b, "%02lld:%02lld", (long long)(s / 3600), (long long)(s / 60 % 60));
		if (s % 60) ey_buf_printf(b, ":%02lld", (long long)(s % 60));
		return;
	}
	case EY_INT:
		ey_int_write(b, WORDOF(p));
		return;
	case EY_STRING: {
		ey_str *s = EY_STR(WORDOF(p));
		if (nested)
			quote(b, s);
		else
			ey_buf_add(b, s->b, s->len);
		return;
	}
	case EY_BYTES: {
		ey_str *s = EY_STR(WORDOF(p));
		ey_buf_char(b, '[');
		for (int64_t i = 0; i < s->len; i++) {
			if (i) ey_buf_str(b, ", ");
			ey_buf_u64(b, (unsigned char)s->b[i], 0);
		}
		ey_buf_char(b, ']');
		return;
	}
	case EY_INSTANT:
		ey_instant_write(b, WORDOF(p));
		return;
	case EY_LIST: {
		ey_list *l = EY_LIST(WORDOF(p));
		ey_buf_char(b, '[');
		show_items(b, t->args[0], l->items, l->len);
		ey_buf_char(b, ']');
		return;
	}
	case EY_SET: {
		ey_map *m = EY_MAP(WORDOF(p));
		if (m->len == 0) {
			ey_buf_str(b, "{}");
			return;
		}
		ey_buf_char(b, '{');
		show_items(b, t->args[0], m->keys, m->len);
		ey_buf_char(b, '}');
		return;
	}
	case EY_MAP: {
		ey_map *m = EY_MAP(WORDOF(p));
		if (m->len == 0) {
			ey_buf_str(b, "{}");
			return;
		}
		ey_buf_str(b, "{ ");
		for (int64_t i = 0; i < m->len; i++) {
			if (i > 0) ey_buf_str(b, ", ");
			ey_show_to(b, t->args[0], m->keys + i * (int64_t)t->args[0]->size, true);
			ey_buf_str(b, ": ");
			ey_show_to(b, t->args[1], m->vals + i * (int64_t)t->args[1]->size, true);
		}
		ey_buf_str(b, " }");
		return;
	}
	case EY_TUPLE:
		ey_buf_char(b, '(');
		for (uint32_t i = 0; i < t->n; i++) {
			if (i > 0) ey_buf_str(b, ", ");
			ey_show_to(b, t->args[i], ey_field(t, p, i), true);
		}
		ey_buf_char(b, ')');
		return;
	case EY_RECORD:
		ey_buf_str(b, "{ ");
		for (uint32_t i = 0; i < t->n; i++) {
			if (i > 0) ey_buf_str(b, ", ");
			ey_buf_str(b, t->names[i]);
			ey_buf_str(b, ": ");
			ey_show_to(b, t->args[i], ey_field(t, p, i), true);
		}
		ey_buf_str(b, " }");
		return;
	case EY_VARIANT: {
		uint32_t tag = ey_variant_of(t, p);
		const ey_variant *vr = &t->variants[tag];
		ey_buf_str(b, vr->name);
		if (vr->n == 0) return;
		ey_buf_char(b, '(');
		for (uint32_t i = 0; i < vr->n; i++) {
			if (i > 0) ey_buf_str(b, ", ");
			if (vr->names[i][0]) {
				ey_buf_str(b, vr->names[i]);
				ey_buf_str(b, ": ");
			}
			ey_show_to(b, vr->fields[i], ey_variant_field(t, p, tag, i), true);
		}
		ey_buf_char(b, ')');
		return;
	}
	case EY_FN:
		ey_buf_str(b, "fn ");
		ey_buf_str(b, t->name ? t->name : "lambda");
		return;
	case EY_DYN: {
		ey_dyn *d = EY_DYNP(WORDOF(p));
		ey_show_to(b, d->h.ty->args[0], EY_DYN_VAL(d), nested);
		return;
	}
	case EY_RANGE: {
		ey_range_obj *r = EY_RANGEP(WORDOF(p));
		if (!(r->flags & EY_RANGE_NO_START)) ey_show_to(b, t->args[0], &r->start, true);
		ey_buf_str(b, r->flags & EY_RANGE_INCLUSIVE ? "..=" : "..");
		if (!(r->flags & EY_RANGE_NO_END)) ey_show_to(b, t->args[0], &r->end, true);
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

ey_v ey_show(const ey_type *t, const void *p) {
	if (t->show) return t->show(p);
	switch (t->kind) {
	case EY_INT:
		return ey_show_int(WORDOF(p));
	case EY_INT32:
	case EY_INT64: {
		char s[24];
		char *q = ey_fmt_i64(s + sizeof s, (int64_t)ey_load_v(t, p));
		return ey_str_new(q, s + sizeof s - q, &ey_t_string);
	}
	case EY_BYTE:
	case EY_UINT32:
	case EY_UINT64: {
		char s[24];
		char *q = ey_fmt_u64(s + sizeof s, ey_load_v(t, p));
		return ey_str_new(q, s + sizeof s - q, &ey_t_string);
	}
	case EY_STRING:
		ey_dup(WORDOF(p)); /* a String shows as itself */
		return WORDOF(p);
	default:
		break;
	}
	ey_buf b = {0};
	ey_show_to(&b, t, p, false);
	return ey_buf_take(&b);
}
