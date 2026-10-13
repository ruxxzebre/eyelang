/* Saved values: the bytes entity state, a job's arguments and steps, and
 * every value the Go runner hands to compiled code or takes from it are
 * kept as (ROADMAP 7.6, 7.9). They are the interpreter's encoding,
 * interp/save.go, byte for byte, so that a value one side saves the other
 * reads, and a journal or a Postgres store holds the same bytes whichever
 * side wrote them.
 *
 * The encoding is tagged: a byte says what follows. Numbers are Go's
 * varints (encoding/binary: zigzag for signed ones), a string its length
 * and its bytes.
 *
 *   i varint            an Int that fits in 64 bits (the fixed-width
 *                       integers too)
 *   I string            a larger Int, in decimal
 *   f uvarint           a Float's bits
 *   s string            a String, Key or Zone
 *   T, F, u             true, false, ()
 *   c varint            a Char
 *   d varint, z varint  a Duration in nanoseconds, a Size in bytes
 *   D y m d, t h m s    a Date, a Time, each part a varint
 *   W varint string     an Instant: nanoseconds since 1970 and its zone
 *   L n values          a List, and Bytes as a list of Ints
 *   S n values          a Set
 *   M n keys n values   a Map
 *   E                   {} before anything decided what it is
 *   P n values          a tuple
 *   N type n (name value)*   a record of a declared type
 *   R n (name value)*        a record of none
 *   V variant n values  a variant, by its module, type and name
 *   g flags [start] [end]    a range: 1 inclusive, 2 a start, 4 an end
 *
 * The back end writes an encoder and a decoder for each type that is saved
 * (cgen/save.go), from the low IR's monomorphic types; these are what they
 * share. Every static name here starts with sv_, since at -O2 the runtime
 * is one translation unit. */
#include <stdio.h>
#include <stdlib.h>

#include "eye.h"
#include "builtins.h"

/* ---------------------------------------------------------------- writing */

void eys_uvarint(ey_buf *b, uint64_t n) {
	char buf[10];
	int k = 0;
	while (n >= 0x80) {
		buf[k++] = (char)(n | 0x80);
		n >>= 7;
	}
	buf[k++] = (char)n;
	ey_buf_add(b, buf, k);
}

void eys_varint(ey_buf *b, int64_t n) {
	uint64_t u = (uint64_t)n << 1;
	if (n < 0) u = ~u;
	eys_uvarint(b, u);
}

void eys_text(ey_buf *b, const char *s, int64_t n) {
	eys_uvarint(b, (uint64_t)n);
	ey_buf_add(b, s, n);
}

void eys_tagged(ey_buf *b, char tag, int64_t n) {
	ey_buf_char(b, tag);
	eys_varint(b, n);
}

void eys_int(ey_buf *b, ey_v n) {
	int64_t v;
	if (ey_int_i64_of(n, &v)) {
		eys_tagged(b, 'i', v);
		return;
	}
	ey_buf d = {0};
	ey_int_write(&d, n);
	ey_buf_char(b, 'I');
	eys_text(b, d.b, d.len);
	free(d.b);
}

void eys_fixed(ey_buf *b, const ey_type *t, const void *p) {
	uint64_t v = ey_load_v(t, p);
	if (t->kind == EY_UINT64 && v > (uint64_t)INT64_MAX) {
		char d[24];
		snprintf(d, sizeof d, "%llu", (unsigned long long)v);
		ey_buf_char(b, 'I');
		eys_text(b, d, (int64_t)strlen(d));
		return;
	}
	eys_tagged(b, 'i', (int64_t)v);
}

void eys_float(ey_buf *b, double d) {
	uint64_t bits;
	memcpy(&bits, &d, 8);
	ey_buf_char(b, 'f');
	eys_uvarint(b, bits);
}

void eys_string(ey_buf *b, ey_v s) {
	ey_buf_char(b, 's');
	eys_text(b, EY_STR(s)->b, EY_STR(s)->len);
}

void eys_bytes(ey_buf *b, ey_v s) {
	ey_str *x = EY_STR(s);
	ey_buf_char(b, 'L');
	eys_uvarint(b, (uint64_t)x->len);
	for (int64_t i = 0; i < x->len; i++) eys_tagged(b, 'i', (unsigned char)x->b[i]);
}

void eys_date(ey_buf *b, int64_t days) {
	int64_t y, m, d;
	ey_civil_from_days(days, &y, &m, &d);
	ey_buf_char(b, 'D');
	eys_varint(b, y);
	eys_varint(b, m);
	eys_varint(b, d);
}

void eys_time(ey_buf *b, int64_t secs) {
	ey_buf_char(b, 't');
	eys_varint(b, secs / 3600);
	eys_varint(b, secs / 60 % 60);
	eys_varint(b, secs % 60);
}

void eys_instant(ey_buf *b, ey_v v) {
	ey_instant *t = (ey_instant *)(uintptr_t)v;
	const char *zone = t->zone ? t->zone->name : "UTC";
	eys_tagged(b, 'W', t->ns);
	eys_text(b, zone, (int64_t)strlen(zone));
}

static void sv_items(ey_buf *b, const char *items, int64_t n, int64_t size, ey_encoder e) {
	eys_uvarint(b, (uint64_t)n);
	for (int64_t i = 0; i < n; i++) e(b, items + i * size);
}

void eys_list(ey_buf *b, ey_v l, ey_encoder e) {
	ey_list *x = EY_LIST(l);
	ey_buf_char(b, 'L');
	sv_items(b, x->items, x->len, x->h.ty->args[0]->size, e);
}

void eys_set(ey_buf *b, ey_v s, ey_encoder e) {
	ey_map *x = EY_MAP(s);
	ey_buf_char(b, 'S');
	sv_items(b, x->keys, x->len, x->h.ty->args[0]->size, e);
}

void eys_map(ey_buf *b, ey_v m, ey_encoder ke, ey_encoder ve) {
	ey_map *x = EY_MAP(m);
	ey_buf_char(b, 'M');
	sv_items(b, x->keys, x->len, x->h.ty->args[0]->size, ke);
	sv_items(b, x->vals, x->len, x->h.ty->args[1]->size, ve);
}

void eys_range(ey_buf *b, ey_v r, const ey_type *elem, ey_encoder e) {
	ey_range_obj *x = EY_RANGEP(r);
	uint8_t flags = 0;
	if (x->flags & EY_RANGE_INCLUSIVE) flags |= 1;
	if (!(x->flags & EY_RANGE_NO_START)) flags |= 2;
	if (!(x->flags & EY_RANGE_NO_END)) flags |= 4;
	ey_buf_char(b, 'g');
	ey_buf_char(b, (char)flags);
	ey_tmp v;
	if (flags & 2) {
		ey_store_v(elem, &v, x->start);
		e(b, &v);
	}
	if (flags & 4) {
		ey_store_v(elem, &v, x->end);
		e(b, &v);
	}
}

/* ---------------------------------------------------------------- reading */

bool eys_fail(ey_rd *r, const char *why) {
	if (!r->err) r->err = why;
	return false;
}

static bool sv_corrupt(ey_rd *r) { return eys_fail(r, "corrupt saved value"); }

bool eys_tag(ey_rd *r, char want) {
	if (r->p >= r->end) return sv_corrupt(r);
	if (*r->p != (uint8_t)want) return eys_fail(r, "the saved value isn't of the type it is read as");
	r->p++;
	return true;
}

bool eys_read_uvarint(ey_rd *r, uint64_t *out) {
	uint64_t n = 0;
	for (int shift = 0; shift < 70; shift += 7) {
		if (r->p >= r->end) return sv_corrupt(r);
		uint8_t c = *r->p++;
		if (shift == 63 && c > 1) return sv_corrupt(r);
		n |= (uint64_t)(c & 0x7f) << shift;
		if (c < 0x80) {
			*out = n;
			return true;
		}
	}
	return sv_corrupt(r);
}

bool eys_read_varint(ey_rd *r, int64_t *out) {
	uint64_t u;
	if (!eys_read_uvarint(r, &u)) return false;
	int64_t n = (int64_t)(u >> 1);
	if (u & 1) n = ~n;
	*out = n;
	return true;
}

bool eys_read_text(ey_rd *r, const char **s, int64_t *n) {
	uint64_t len;
	if (!eys_read_uvarint(r, &len)) return false;
	if (len > (uint64_t)(r->end - r->p)) return sv_corrupt(r);
	*s = (const char *)r->p;
	*n = (int64_t)len;
	r->p += len;
	return true;
}

bool eys_text_is(const char *s, int64_t n, const char *want) {
	return (int64_t)strlen(want) == n && memcmp(s, want, (size_t)n) == 0;
}

bool eys_read_tagged(ey_rd *r, char tag, int64_t *out) { return eys_tag(r, tag) && eys_read_varint(r, out); }

bool eys_read_int(ey_rd *r, ey_v *out) {
	if (r->p >= r->end) return sv_corrupt(r);
	if (*r->p == 'i') {
		int64_t v;
		if (!eys_read_tagged(r, 'i', &v)) return false;
		*out = ey_int_i64(v);
		return true;
	}
	const char *s;
	int64_t n;
	if (!eys_tag(r, 'I') || !eys_read_text(r, &s, &n)) return false;
	if (!ey_int_parse(s, n, 10, out)) return sv_corrupt(r);
	return true;
}

bool eys_read_fixed(ey_rd *r, const ey_type *t, void *out) {
	static const struct {
		int kind;
		int64_t lo, hi;
	} ranges[] = {{EY_BYTE, 0, 255}, {EY_INT32, INT32_MIN, INT32_MAX}, {EY_UINT32, 0, UINT32_MAX}, {EY_INT64, INT64_MIN, INT64_MAX}, {EY_UINT64, 0, 0}};
	ey_v n;
	if (!eys_read_int(r, &n)) return false;
	for (int i = 0; i < 5; i++) {
		if (ranges[i].kind != (int)t->kind) continue;
		bool fits = ey_int_fits(n, ranges[i].lo, ranges[i].hi, t->kind == EY_UINT64);
		uint64_t bits = ey_int_bits(n);
		ey_drop(n);
		if (!fits) return eys_fail(r, "the saved number doesn't fit its type");
		ey_store_v(t, out, bits);
		return true;
	}
	ey_drop(n);
	return sv_corrupt(r);
}

bool eys_read_float(ey_rd *r, double *out) {
	uint64_t bits;
	if (!eys_tag(r, 'f') || !eys_read_uvarint(r, &bits)) return false;
	memcpy(out, &bits, 8);
	return true;
}

bool eys_read_bool(ey_rd *r, bool *out) {
	if (r->p >= r->end) return sv_corrupt(r);
	if (*r->p != 'T' && *r->p != 'F') return eys_fail(r, "the saved value isn't of the type it is read as");
	*out = *r->p++ == 'T';
	return true;
}

bool eys_read_string(ey_rd *r, ey_v *out) {
	const char *s;
	int64_t n;
	if (!eys_tag(r, 's') || !eys_read_text(r, &s, &n)) return false;
	*out = ey_str_new(s, n, &ey_t_string);
	return true;
}

bool eys_read_bytes(ey_rd *r, ey_v *out) {
	uint64_t n;
	if (!eys_tag(r, 'L') || !eys_read_uvarint(r, &n)) return false;
	if (n > (uint64_t)(r->end - r->p)) return sv_corrupt(r);
	ey_buf b = {0};
	for (uint64_t i = 0; i < n; i++) {
		int64_t v;
		if (!eys_read_tagged(r, 'i', &v) || v < 0 || v > 255) {
			free(b.b);
			return eys_fail(r, "the saved bytes aren't Bytes");
		}
		ey_buf_char(&b, (char)v);
	}
	*out = ey_str_new(b.b ? b.b : "", b.len, &ey_t_bytes);
	free(b.b);
	return true;
}

bool eys_read_date(ey_rd *r, int64_t *days) {
	int64_t y, m, d;
	if (!eys_tag(r, 'D') || !eys_read_varint(r, &y) || !eys_read_varint(r, &m) || !eys_read_varint(r, &d)) return false;
	*days = ey_days_from_civil(y, m, d);
	return true;
}

bool eys_read_time(ey_rd *r, int64_t *secs) {
	int64_t h, m, s;
	if (!eys_tag(r, 't') || !eys_read_varint(r, &h) || !eys_read_varint(r, &m) || !eys_read_varint(r, &s)) return false;
	*secs = h * 3600 + m * 60 + s;
	return true;
}

bool eys_read_instant(ey_rd *r, ey_v *out) {
	int64_t ns;
	const char *s;
	int64_t n;
	if (!eys_read_tagged(r, 'W', &ns) || !eys_read_text(r, &s, &n)) return false;
	const ey_zone *z = NULL;
	if (n > 0 && !eys_text_is(s, n, "UTC")) {
		/* a zone the program names: the back end wrote its table */
		ey_v name = ey_str_new(s, n, &ey_t_string);
		z = ey_zone_find("a saved Instant", name);
		ey_drop(name);
	}
	*out = ey_instant_new(ns, z);
	return true;
}

/* sv_elems reads a count and that many values of type t into a new array */
static bool sv_elems(ey_rd *r, const ey_type *t, ey_decoder d, int64_t *n, char **items) {
	uint64_t count;
	if (!eys_read_uvarint(r, &count)) return false;
	if (count > (uint64_t)(r->end - r->p)) return sv_corrupt(r); /* each value takes a byte at least */
	int64_t size = t->size ? t->size : 1;
	char *a = calloc(count ? count : 1, (size_t)size);
	if (!a) ey_out_of_memory();
	for (uint64_t i = 0; i < count; i++) {
		if (!d(r, a + i * size)) {
			ey_drop_n(t, a, (int64_t)i);
			free(a);
			return false;
		}
	}
	*n = (int64_t)count;
	*items = a;
	return true;
}

/* sv_ptrs points at each of n values of size bytes */
static void **sv_ptrs(char *items, int64_t n, int64_t size) {
	void **ps = malloc((size_t)(n ? n : 1) * sizeof *ps);
	if (!ps) ey_out_of_memory();
	for (int64_t i = 0; i < n; i++) ps[i] = items + i * size;
	return ps;
}

bool eys_read_list(ey_rd *r, const ey_type *lt, ey_decoder d, ey_v *out) {
	int64_t n;
	char *items;
	if (!eys_tag(r, 'L') || !sv_elems(r, lt->args[0], d, &n, &items)) return false;
	void **ps = sv_ptrs(items, n, lt->args[0]->size);
	*out = ey_list_from(lt, n, (void *const *)ps);
	free(ps);
	free(items);
	return true;
}

/* {} saved before anything decided it was a map or a set */
static bool sv_empty(ey_rd *r) {
	if (r->p < r->end && *r->p == 'E') {
		r->p++;
		return true;
	}
	return false;
}

bool eys_read_set(ey_rd *r, const ey_type *st, ey_decoder d, ey_v *out) {
	if (sv_empty(r)) {
		*out = ey_set_from(st, 0, NULL);
		return true;
	}
	int64_t n;
	char *items;
	if (!eys_tag(r, 'S') || !sv_elems(r, st->args[0], d, &n, &items)) return false;
	void **ps = sv_ptrs(items, n, st->args[0]->size);
	*out = ey_set_from(st, n, (void *const *)ps);
	free(ps);
	free(items);
	return true;
}

bool eys_read_map(ey_rd *r, const ey_type *mt, ey_decoder kd, ey_decoder vd, ey_v *out) {
	if (sv_empty(r)) {
		*out = ey_map_from(mt, 0, NULL, NULL);
		return true;
	}
	int64_t n, nv;
	char *keys, *vals;
	if (!eys_tag(r, 'M') || !sv_elems(r, mt->args[0], kd, &n, &keys)) return false;
	if (!sv_elems(r, mt->args[1], vd, &nv, &vals)) {
		ey_drop_n(mt->args[0], keys, n);
		free(keys);
		return false;
	}
	if (nv != n) {
		ey_drop_n(mt->args[0], keys, n);
		ey_drop_n(mt->args[1], vals, nv);
		free(keys);
		free(vals);
		return sv_corrupt(r);
	}
	void **kp = sv_ptrs(keys, n, mt->args[0]->size), **vp = sv_ptrs(vals, n, mt->args[1]->size);
	*out = ey_map_from(mt, n, (void *const *)kp, (void *const *)vp);
	free(kp);
	free(vp);
	free(keys);
	free(vals);
	return true;
}

bool eys_read_range(ey_rd *r, const ey_type *rt, ey_decoder d, ey_v *out) {
	if (!eys_tag(r, 'g')) return false;
	if (r->p >= r->end) return sv_corrupt(r);
	uint8_t flags = *r->p++;
	const ey_type *elem = rt->args[0];
	ey_v start = 0, end = 0;
	ey_tmp v;
	if (flags & 2) {
		memset(&v, 0, sizeof v);
		if (!d(r, &v)) return false;
		start = ey_load_v(elem, &v);
	}
	if (flags & 4) {
		memset(&v, 0, sizeof v);
		if (!d(r, &v)) {
			if (ey_counted(elem)) ey_drop(start);
			return false;
		}
		end = ey_load_v(elem, &v);
	}
	uint32_t f = 0;
	if (flags & 1) f |= EY_RANGE_INCLUSIVE;
	if (!(flags & 2)) f |= EY_RANGE_NO_START;
	if (!(flags & 4)) f |= EY_RANGE_NO_END;
	*out = ey_range(rt, start, end, f);
	return true;
}

bool eys_read_record(ey_rd *r, uint64_t *n) {
	if (r->p >= r->end) return sv_corrupt(r);
	if (*r->p == 'N') {
		r->p++;
		const char *s;
		int64_t len;
		if (!eys_read_text(r, &s, &len)) return false; /* the type's name; the decoder knows its type */
	} else if (!eys_tag(r, 'R')) {
		return false;
	}
	return eys_read_uvarint(r, n);
}

bool eys_read_variant(ey_rd *r, const char **name, int64_t *len, uint64_t *n) {
	return eys_tag(r, 'V') && eys_read_text(r, name, len) && eys_read_uvarint(r, n);
}

/* ---------------------------------------------------------------- the runner's entry points */

const ey_saved *ey_saved_find(const char *name) {
	for (int i = 0; i < ey_nsaved; i++)
		if (strcmp(ey_saved_types[i].name, name) == 0) return &ey_saved_types[i];
	return NULL;
}

void ey_save(const ey_saved *s, const void *v, ey_buf *out) { s->encode(out, v); }

const char *ey_load(const ey_saved *s, const char *data, int64_t n, void *out) {
	ey_rd r = {(const uint8_t *)data, (const uint8_t *)data + n, NULL};
	memset(out, 0, s->ty->size);
	if (!s->decode(&r, out)) return r.err ? r.err : "corrupt saved value";
	if (r.p != r.end) {
		ey_drop_at(s->ty, out);
		memset(out, 0, s->ty->size);
		return "the saved value has bytes left over";
	}
	return NULL;
}

/* ---------------------------------------------------------------- checking, for cgen's tests */

static void sv_hex(const char *s, int64_t n) {
	for (int64_t i = 0; i < n; i++) printf("%02x", (unsigned char)s[i]);
}

static int sv_unhex(int c) {
	if (c >= '0' && c <= '9') return c - '0';
	if (c >= 'a' && c <= 'f') return c - 'a' + 10;
	return -1;
}

/* ey_save_check is the main of a binary built to check saving: with
 * EYE_SAVE_CHECK=save it writes each probe's name and its value's bytes in
 * hex; with load, it reads lines of a name and bytes from stdin, decodes
 * them as that probe's type, and writes the name, the bytes it encodes
 * the value back to, whether the value equals the probe's, and how it
 * shows, in hex. */
void ey_save_check(const ey_probe *ps, int n) {
	const char *mode = getenv("EYE_SAVE_CHECK");
	if (mode && strcmp(mode, "load") == 0) {
		static char line[1 << 20];
		while (fgets(line, sizeof line, stdin)) {
			char *sp = strchr(line, ' ');
			if (!sp) continue;
			*sp = 0;
			const ey_probe *p = NULL;
			for (int i = 0; i < n; i++)
				if (strcmp(ps[i].name, line) == 0) p = &ps[i];
			if (!p) continue;
			ey_buf data = {0};
			for (char *h = sp + 1; sv_unhex(h[0]) >= 0 && sv_unhex(h[1]) >= 0; h += 2) ey_buf_char(&data, (char)(sv_unhex(h[0]) * 16 + sv_unhex(h[1])));
			const ey_saved *s = &ey_saved_types[p->saved];
			ey_tmp *v = calloc(1, s->ty->size > sizeof(ey_tmp) ? s->ty->size : sizeof(ey_tmp));
			const char *err = ey_load(s, data.b ? data.b : "", data.len, v);
			printf("%s ", p->name);
			if (err) {
				printf("error %s\n", err);
			} else {
				ey_buf again = {0};
				ey_save(s, v, &again);
				sv_hex(again.b, again.len);
				printf(" %d ", ey_eq(s->ty, v, p->value()));
				ey_v shown = ey_show(s->ty, v);
				sv_hex(EY_STR(shown)->b, EY_STR(shown)->len);
				printf("\n");
				ey_drop(shown);
				free(again.b);
				ey_drop_at(s->ty, v);
			}
			free(v);
			free(data.b);
		}
		return;
	}
	for (int i = 0; i < n; i++) {
		ey_buf b = {0};
		ey_save(&ey_saved_types[ps[i].saved], ps[i].value(), &b);
		printf("%s ", ps[i].name);
		sv_hex(b.b, b.len);
		printf("\n");
		free(b.b);
	}
}
