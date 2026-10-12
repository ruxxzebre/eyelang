/* Strings (UTF-8) and Bytes, characters, positions in strings (Offset,
 * SPECS.md §5) and the numbers read from text. An Offset is a newtype of
 * Int, so it is a small Int here: a byte offset. */
#include <errno.h>
#include <math.h>
#include <stdio.h>
#include <stdlib.h>

#include "eye.h"
#include "builtins.h"

ey_v ey_str_new(const char *s, int64_t n, const ey_type *ty) {
	ey_str *p = ey_alloc(sizeof(ey_str) + (size_t)n + 1, ty);
	p->len = n;
	p->cap = 0;
	if (n) memcpy(p->b, s, (size_t)n);
	p->b[n] = 0;
	return EY_V(p);
}

ey_v ey_concat(int n, const ey_v *parts) {
	int64_t len = 0;
	for (int i = 0; i < n; i++) len += EY_STR(parts[i])->len;
	ey_str *p = ey_alloc(sizeof(ey_str) + (size_t)len + 1, &ey_t_string);
	p->len = len;
	p->cap = 0;
	char *at = p->b;
	for (int i = 0; i < n; i++) {
		memcpy(at, EY_STR(parts[i])->b, (size_t)EY_STR(parts[i])->len);
		at += EY_STR(parts[i])->len;
	}
	*at = 0;
	return EY_V(p);
}

ey_v eyo_concat(ey_v sv, int n, const ey_v *parts) {
	ey_str *s = EY_STR(sv);
	int64_t add = 0;
	bool alone = s->h.rc == 1;
	for (int i = 0; i < n; i++) {
		add += EY_STR(parts[i])->len;
		alone = alone && parts[i] != sv;
	}
	if (!alone) {
		ey_v *all = malloc(sizeof(ey_v) * (size_t)(n + 1));
		if (!all) {
			fputs("out of memory\n", stderr);
			exit(2);
		}
		all[0] = sv;
		memcpy(all + 1, parts, sizeof(ey_v) * (size_t)n);
		ey_v r = ey_concat(n + 1, all);
		EY_OBJ(r)->ty = s->h.ty; /* a String or Bytes */
		free(all);
		ey_drop(sv);
		return r;
	}
	int64_t need = s->len + add + 1;
	int64_t room = s->cap ? s->cap : s->len + 1;
	if (need > room) {
		int64_t cap = room * 2 > need ? room * 2 : need;
		if (cap < 32) cap = 32;
		s = realloc(s, sizeof(ey_str) + (size_t)cap);
		if (!s) {
			fputs("out of memory\n", stderr);
			exit(2);
		}
		s->cap = cap;
	}
	for (int i = 0; i < n; i++) {
		ey_str *p = EY_STR(parts[i]);
		memcpy(s->b + s->len, p->b, (size_t)p->len);
		s->len += p->len;
	}
	s->b[s->len] = 0;
	return EY_V(s);
}

ey_v eyo_str_append(ey_v s, ey_v t) { return eyo_concat(s, 1, &t); }

bool ey_str_eq(ey_v a, ey_v b) {
	ey_str *x = EY_STR(a), *y = EY_STR(b);
	return x->len == y->len && memcmp(x->b, y->b, (size_t)x->len) == 0;
}

int ey_str_cmp(ey_v a, ey_v b) {
	ey_str *x = EY_STR(a), *y = EY_STR(b);
	int64_t n = x->len < y->len ? x->len : y->len;
	int c = memcmp(x->b, y->b, (size_t)n);
	if (c != 0) return c < 0 ? -1 : 1;
	return x->len < y->len ? -1 : x->len > y->len;
}

/* ---------------------------------------------------------------- UTF-8 */

int64_t ey_utf8_decode(const char *s, int64_t n, uint32_t *r) {
	const unsigned char *u = (const unsigned char *)s;
	*r = 0xfffd;
	if (n <= 0) return 0;
	if (u[0] < 0x80) {
		*r = u[0];
		return 1;
	}
	int need;
	uint32_t c, min;
	if ((u[0] & 0xe0) == 0xc0) {
		need = 1, c = u[0] & 0x1f, min = 0x80;
	} else if ((u[0] & 0xf0) == 0xe0) {
		need = 2, c = u[0] & 0x0f, min = 0x800;
	} else if ((u[0] & 0xf8) == 0xf0) {
		need = 3, c = u[0] & 0x07, min = 0x10000;
	} else {
		return 1;
	}
	if (n < need + 1) return 1;
	for (int i = 1; i <= need; i++) {
		if ((u[i] & 0xc0) != 0x80) return 1;
		c = (c << 6) | (u[i] & 0x3f);
	}
	if (c < min || c > 0x10ffff || (c >= 0xd800 && c <= 0xdfff)) return 1;
	*r = c;
	return need + 1;
}

bool ey_utf8_valid(const char *s, int64_t n) {
	for (int64_t i = 0; i < n;) {
		uint32_t r;
		int64_t k = ey_utf8_decode(s + i, n - i, &r);
		if (r == 0xfffd && k == 1 && (unsigned char)s[i] >= 0x80) return false;
		i += k;
	}
	return true;
}

/* the length of the character before byte i */
static int64_t last_rune_len(const char *s, int64_t i) {
	int64_t start = i - 1;
	while (start > 0 && i - start < 4 && ((unsigned char)s[start] & 0xc0) == 0x80) start--;
	uint32_t r;
	int64_t k = ey_utf8_decode(s + start, i - start, &r);
	if (start + k != i) return 1;
	return k;
}

static bool char_start(ey_str *s, int64_t i) {
	return i >= 0 && (i == s->len || (i < s->len && ((unsigned char)s->b[i] & 0xc0) != 0x80));
}

/* the byte offset after the first n characters of s, or its length */
static int64_t char_offset(ey_str *s, int64_t n) {
	int64_t i = 0;
	while (i < s->len && n > 0) {
		uint32_t r;
		i += ey_utf8_decode(s->b + i, s->len - i, &r);
		n--;
	}
	return i;
}

static int64_t count(const char *at, ey_v n) {
	int64_t c;
	if (!ey_int_i64_of(n, &c) || c < 0) ey_fault(at, "a count is an Int that isn't negative");
	return c;
}

/* ---------------------------------------------------------------- String */

ey_v eyb_str_size(ey_v s) {
	ey_str *p = EY_STR(s);
	int64_t n = 0;
	for (int64_t i = 0; i < p->len;) {
		uint32_t r;
		i += ey_utf8_decode(p->b + i, p->len - i, &r);
		n++;
	}
	return EY_SMALL(n);
}

ey_v eyb_str_take(const char *at, ey_v s, ey_v n) {
	ey_str *p = EY_STR(s);
	return ey_str_new(p->b, char_offset(p, count(at, n)), &ey_t_string);
}

ey_v eyb_str_drop(const char *at, ey_v s, ey_v n) {
	ey_str *p = EY_STR(s);
	int64_t i = char_offset(p, count(at, n));
	return ey_str_new(p->b + i, p->len - i, &ey_t_string);
}

static int64_t find(ey_str *s, int64_t from, ey_str *part) {
	if (part->len == 0) return from;
	for (int64_t i = from; i + part->len <= s->len; i++)
		if (memcmp(s->b + i, part->b, (size_t)part->len) == 0) return i;
	return -1;
}

ey_v eyb_str_index_of(ey_v s, ey_v part, ey_v from, bool has_from, const ey_type *opt) {
	ey_str *p = EY_STR(s);
	int64_t f = 0;
	if (has_from) {
		int64_t o = EY_IS_SMALL(from) ? EY_SMALL_VAL(from) : 0;
		f = o < 0 ? 0 : o > p->len ? p->len : o;
	}
	int64_t i = find(p, f, EY_STR(part));
	if (i < 0) return ey_none(opt);
	return ey_some(opt, EY_SMALL(i));
}

static int64_t clamp_offset(ey_v o, int64_t n) {
	int64_t v = EY_IS_SMALL(o) ? EY_SMALL_VAL(o) : 0;
	return v < 0 ? 0 : v > n ? n : v;
}

ey_v eyb_str_slice(ey_v s, ey_v range, const ey_type *opt) {
	ey_str *p = EY_STR(s);
	ey_agg *r = EY_AGG(range);
	int64_t lo = r->f[2] & EY_RANGE_NO_START ? 0 : clamp_offset(r->f[0], p->len);
	int64_t hi = r->f[2] & EY_RANGE_NO_END ? p->len : clamp_offset(r->f[1], p->len);
	if ((r->f[2] & EY_RANGE_INCLUSIVE) && !(r->f[2] & EY_RANGE_NO_END) && hi < p->len) {
		if (!char_start(p, hi)) return ey_none(opt);
		uint32_t c;
		hi += ey_utf8_decode(p->b + hi, p->len - hi, &c);
	}
	if (hi < lo) return ey_some(opt, ey_str_new("", 0, &ey_t_string));
	if (!char_start(p, lo) || !char_start(p, hi)) return ey_none(opt);
	return ey_some(opt, ey_str_new(p->b + lo, hi - lo, &ey_t_string));
}

ey_v eyb_str_step(const char *at, ey_v s, ey_v off, ey_v by, bool has_by, bool forward, const ey_type *opt) {
	ey_str *p = EY_STR(s);
	int64_t i = EY_IS_SMALL(off) ? EY_SMALL_VAL(off) : -1;
	int64_t n = has_by ? count(at, by) : 1;
	if (!char_start(p, i)) return ey_none(opt);
	for (; n > 0; n--) {
		if (forward) {
			if (i == p->len) return ey_none(opt);
			uint32_t r;
			i += ey_utf8_decode(p->b + i, p->len - i, &r);
		} else {
			if (i == 0) return ey_none(opt);
			i -= last_rune_len(p->b, i);
		}
	}
	return ey_some(opt, EY_SMALL(i));
}

ey_v eyb_str_char_at(ey_v s, ey_v off, const ey_type *opt) {
	ey_str *p = EY_STR(s);
	int64_t i = EY_IS_SMALL(off) ? EY_SMALL_VAL(off) : -1;
	if (i == p->len || !char_start(p, i)) return ey_none(opt);
	uint32_t r;
	ey_utf8_decode(p->b + i, p->len - i, &r);
	return ey_some(opt, r);
}

ey_v eyb_str_replace(ey_v s, ey_v a, ey_v b) {
	ey_str *p = EY_STR(s), *x = EY_STR(a), *y = EY_STR(b);
	if (x->len == 0) {
		ey_dup(s);
		return s;
	}
	ey_buf out = {0};
	int64_t i = 0;
	for (;;) {
		int64_t j = find(p, i, x);
		if (j < 0) break;
		ey_buf_add(&out, p->b + i, j - i);
		ey_buf_add(&out, y->b, y->len);
		i = j + x->len;
	}
	ey_buf_add(&out, p->b + i, p->len - i);
	return ey_buf_take(&out);
}

/* whether r is white space, as Go's unicode.IsSpace says */
static bool is_space(uint32_t r) {
	switch (r) {
	case '\t': case '\n': case '\v': case '\f': case '\r': case ' ': case 0x85: case 0xa0:
	case 0x1680: case 0x2028: case 0x2029: case 0x202f: case 0x205f: case 0x3000:
		return true;
	}
	return r >= 0x2000 && r <= 0x200a;
}

static ey_v list_of(const ey_type *ty, ey_v *items, int64_t n) { return ey_list_take(ty, n, items); }

static void push(ey_v **items, int64_t *n, int64_t *cap, ey_v v) {
	if (*n == *cap) {
		*cap = *cap ? *cap * 2 : 8;
		*items = realloc(*items, sizeof(ey_v) * (size_t)*cap);
	}
	(*items)[(*n)++] = v;
}

ey_v eyb_str_words(ey_v s, const ey_type *list) {
	ey_str *p = EY_STR(s);
	ey_v *items = NULL;
	int64_t n = 0, cap = 0, start = -1;
	for (int64_t i = 0; i <= p->len;) {
		uint32_t r = ' ';
		int64_t k = 1;
		if (i < p->len) k = ey_utf8_decode(p->b + i, p->len - i, &r);
		if (is_space(r)) {
			if (start >= 0) push(&items, &n, &cap, ey_str_new(p->b + start, i - start, &ey_t_string));
			start = -1;
		} else if (start < 0) {
			start = i;
		}
		i += k;
	}
	return list_of(list, items, n);
}

ey_v eyb_str_trim(ey_v s) {
	ey_str *p = EY_STR(s);
	int64_t lo = 0, hi = p->len;
	while (lo < hi) {
		uint32_t r;
		int64_t k = ey_utf8_decode(p->b + lo, hi - lo, &r);
		if (!is_space(r)) break;
		lo += k;
	}
	while (hi > lo) {
		int64_t k = last_rune_len(p->b, hi);
		uint32_t r;
		ey_utf8_decode(p->b + hi - k, k, &r);
		if (!is_space(r)) break;
		hi -= k;
	}
	return ey_str_new(p->b + lo, hi - lo, &ey_t_string);
}

bool eyb_str_starts_with(ey_v s, ey_v x) {
	ey_str *p = EY_STR(s), *q = EY_STR(x);
	return q->len <= p->len && memcmp(p->b, q->b, (size_t)q->len) == 0;
}

bool eyb_str_ends_with(ey_v s, ey_v x) {
	ey_str *p = EY_STR(s), *q = EY_STR(x);
	return q->len <= p->len && memcmp(p->b + p->len - q->len, q->b, (size_t)q->len) == 0;
}

bool eyb_str_contains(ey_v s, ey_v x) { return find(EY_STR(s), 0, EY_STR(x)) >= 0; }

ey_v eyb_str_split(ey_v s, ey_v sep, const ey_type *list) {
	ey_str *p = EY_STR(s), *q = EY_STR(sep);
	ey_v *items = NULL;
	int64_t n = 0, cap = 0;
	if (q->len == 0) {
		/* Go splits after each character */
		for (int64_t i = 0; i < p->len;) {
			uint32_t r;
			int64_t k = ey_utf8_decode(p->b + i, p->len - i, &r);
			push(&items, &n, &cap, ey_str_new(p->b + i, k, &ey_t_string));
			i += k;
		}
		return list_of(list, items, n);
	}
	int64_t i = 0;
	for (;;) {
		int64_t j = find(p, i, q);
		if (j < 0) break;
		push(&items, &n, &cap, ey_str_new(p->b + i, j - i, &ey_t_string));
		i = j + q->len;
	}
	push(&items, &n, &cap, ey_str_new(p->b + i, p->len - i, &ey_t_string));
	return list_of(list, items, n);
}

ey_v eyb_str_lines(ey_v s, const ey_type *list) {
	ey_str *p = EY_STR(s);
	ey_v *items = NULL;
	int64_t n = 0, cap = 0, len = p->len;
	if (len == 0) return list_of(list, items, 0);
	if (p->b[len - 1] == '\n') len--;
	int64_t start = 0;
	for (int64_t i = 0; i <= len; i++) {
		if (i == len || p->b[i] == '\n') {
			int64_t end = i;
			if (end > start && p->b[end - 1] == '\r') end--;
			push(&items, &n, &cap, ey_str_new(p->b + start, end - start, &ey_t_string));
			start = i + 1;
		}
	}
	return list_of(list, items, n);
}

ey_v eyb_str_chars(ey_v s, const ey_type *list) {
	ey_str *p = EY_STR(s);
	ey_v *items = NULL;
	int64_t n = 0, cap = 0;
	for (int64_t i = 0; i < p->len;) {
		uint32_t r;
		i += ey_utf8_decode(p->b + i, p->len - i, &r);
		push(&items, &n, &cap, r);
	}
	return list_of(list, items, n);
}

ey_v eyb_str_case(ey_v s, bool upper) {
	ey_str *p = EY_STR(s);
	ey_v r = ey_str_new(p->b, p->len, &ey_t_string);
	ey_str *q = EY_STR(r);
	for (int64_t i = 0; i < q->len; i++) {
		char c = q->b[i];
		if (upper && c >= 'a' && c <= 'z') q->b[i] = (char)(c - 32);
		if (!upper && c >= 'A' && c <= 'Z') q->b[i] = (char)(c + 32);
	}
	return r;
}

ey_v eyb_str_bytes(ey_v s) { return ey_str_new(EY_STR(s)->b, EY_STR(s)->len, &ey_t_bytes); }

/* ---------------------------------------------------------------- characters */

ey_v eyb_from_code(ey_v n, const ey_type *opt) {
	if (!EY_IS_SMALL(n)) return ey_none(opt);
	int64_t c = EY_SMALL_VAL(n);
	if (c < 0 || c > 0x10ffff || (c >= 0xd800 && c <= 0xdfff)) return ey_none(opt);
	return ey_some(opt, (ey_v)c);
}

/* ---------------------------------------------------------------- Bytes */

ey_v eyb_bytes_index(ey_v b, ey_v i, const ey_type *opt) {
	ey_str *p = EY_STR(b);
	if (!EY_IS_SMALL(i)) return ey_none(opt);
	int64_t k = EY_SMALL_VAL(i);
	if (k < 0 || k >= p->len) return ey_none(opt);
	return ey_some(opt, (unsigned char)p->b[k]);
}

ey_v eyb_bytes_at(const char *at, ey_v b, ey_v i) {
	ey_str *p = EY_STR(b);
	int64_t k = EY_IS_SMALL(i) ? EY_SMALL_VAL(i) : -1;
	if (k < 0 || k >= p->len) ey_fault(at, "index out of range");
	return (unsigned char)p->b[k];
}

/* bounds clamps a range of indexes to [0, n], as the interpreter does */
void eyb_bounds(ey_v start, ey_v end, uint32_t flags, int64_t n, int64_t *lo, int64_t *hi) {
	*lo = 0;
	*hi = n;
	if (!(flags & EY_RANGE_NO_START)) {
		int64_t s = EY_IS_SMALL(start) ? EY_SMALL_VAL(start) : (ey_int_cmp(start, EY_SMALL(0)) < 0 ? 0 : n);
		*lo = s < 0 ? 0 : s > n ? n : s;
	}
	if (!(flags & EY_RANGE_NO_END)) {
		int64_t e = EY_IS_SMALL(end) ? EY_SMALL_VAL(end) : (ey_int_cmp(end, EY_SMALL(0)) < 0 ? 0 : n);
		if (flags & EY_RANGE_INCLUSIVE) e++;
		*hi = e < 0 ? 0 : e > n ? n : e;
	}
	if (*hi < *lo) *hi = *lo;
}

ey_v eyb_bytes_slice(ey_v b, ey_v start, ey_v end, uint32_t flags) {
	ey_str *p = EY_STR(b);
	int64_t lo, hi;
	eyb_bounds(start, end, flags, p->len, &lo, &hi);
	return ey_str_new(p->b + lo, hi - lo, p->h.ty);
}

ey_v eyb_bytes_to_text(ey_v b, const ey_type *opt) {
	ey_str *p = EY_STR(b);
	if (!ey_utf8_valid(p->b, p->len)) return ey_none(opt);
	return ey_some(opt, ey_str_new(p->b, p->len, &ey_t_string));
}

/* ---------------------------------------------------------------- numbers from text */

static ey_v err_text(const ey_type *res, const char *what, ey_str *s) {
	ey_buf b = {0};
	ey_buf_str(&b, what);
	ey_show_to(&b, &ey_t_string, EY_V(s), true);
	return ey_err(res, ey_buf_take(&b));
}

ey_v eyb_parse_int(const char *at, ey_v s, ey_v base, bool has_base, const ey_type *res) {
	ey_str *p = EY_STR(s);
	int64_t b = 10;
	if (has_base && (!ey_int_i64_of(base, &b) || b < 2 || b > 36)) ey_fault(at, "parseInt's base: is from 2 to 36");
	/* strings.TrimSpace */
	ey_v t = eyb_str_trim(s);
	ey_v n;
	bool ok = ey_int_parse(EY_STR(t)->b, EY_STR(t)->len, (int)b, &n);
	ey_drop(t);
	if (ok) return ey_ok(res, n);
	if (b == 10) return err_text(res, "not a whole number: ", p);
	char what[64];
	snprintf(what, sizeof what, "not a whole number in base %lld: ", (long long)b);
	return err_text(res, what, p);
}

ey_v eyb_parse_float(ey_v s, const ey_type *res) {
	ey_str *p = EY_STR(s);
	ey_v t = eyb_str_trim(s);
	ey_str *q = EY_STR(t);
	/* Decimal only: no hex floats, no "inf" or "nan", no underscores. */
	bool ok = q->len > 0;
	for (int64_t i = 0; i < q->len; i++)
		if (q->b[i] == 0 || !strchr("0123456789.eE+-", q->b[i])) ok = false;
	double d = 0;
	if (ok) {
		char *end;
		errno = 0;
		d = strtod(q->b, &end);
		if (end != q->b + q->len) ok = false;
		if (ok && errno == ERANGE && isinf(d)) {
			ey_drop(t);
			return err_text(res, "too large for a Float: ", p);
		}
	}
	ey_drop(t);
	if (!ok) return err_text(res, "not a number: ", p);
	return ey_ok(res, ey_f2v(d));
}

ey_v eyb_float_to_int(double d, const ey_type *opt) {
	ey_v n;
	if (!ey_int_from_f64(d, &n)) return ey_none(opt);
	return ey_some(opt, n);
}
