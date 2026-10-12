/* Lists, sets and maps, iteration, and the collection functions. Elements
 * are laid out one after another, each its type's size (eye.h). Sets and
 * maps keep their elements in insertion order, as the interpreter's do, with
 * a hash index beside them. Every function leaves the collections it is
 * given as they were, but the eyo_ updates, which take theirs: an update
 * makes a new one. */
#include <math.h>
#include <stdio.h>
#include <stdlib.h>

#include "eye.h"
#include "builtins.h"

static void *xmalloc(size_t n) {
	void *p = malloc(n ? n : 1);
	if (!p) ey_out_of_memory();
	return p;
}

static void *xrealloc(void *p, size_t n) {
	p = realloc(p, n ? n : 1);
	if (!p) ey_out_of_memory();
	return p;
}

/* grow is the capacity after cap: double while small, half again after,
 * so that a long list holds little room it doesn't use. */
static int64_t grow(int64_t cap) { return cap < 4 ? 4 : cap < 4096 ? cap * 2 : cap + cap / 2; }

/* ---------------------------------------------------------------- lists */

#define ESIZE(l) ((int64_t)(l)->h.ty->args[0]->size)
#define ITEM(base, i, size) ((base) + (i) * (size))

static ey_list *list_alloc(const ey_type *ty, int64_t cap) {
	ey_list *l = ey_alloc(sizeof(ey_list), ty);
	l->len = 0;
	l->cap = cap;
	l->items = xmalloc((size_t)(cap * (int64_t)ty->args[0]->size));
	return l;
}

ey_v ey_list_take(const ey_type *ty, int64_t n, void *items) {
	ey_list *l = ey_alloc(sizeof(ey_list), ty);
	l->len = l->cap = n;
	l->items = items ? items : xmalloc(1);
	return EY_V(l);
}

ey_v ey_list_new(const ey_type *ty, int64_t n, const void *items) {
	ey_list *l = list_alloc(ty, n);
	int64_t es = ty->args[0]->size;
	if (n > 0) memcpy(l->items, items, (size_t)(n * es));
	ey_dup_n(ty->args[0], l->items, n);
	l->len = n;
	return EY_V(l);
}

ey_v ey_list_from(const ey_type *ty, int64_t n, void *const *items) {
	ey_list *l = list_alloc(ty, n);
	int64_t es = ty->args[0]->size;
	for (int64_t i = 0; i < n; i++) memcpy(ITEM(l->items, i, es), items[i], (size_t)es);
	l->len = n;
	return EY_V(l);
}

static void list_reserve(ey_list *l, int64_t need) {
	if (need <= l->cap) return;
	int64_t cap = l->cap;
	while (cap < need) cap = grow(cap);
	l->items = xrealloc(l->items, (size_t)(cap * ESIZE(l)));
	l->cap = cap;
}

/* ---------------------------------------------------------------- sets and maps */

#define KSIZE(m) ((int64_t)(m)->h.ty->args[0]->size)
#define VSIZE(m) ((int64_t)(m)->h.ty->args[1]->size)

/* copy copies a value of n bytes; a word, the common case, in one move */
static inline void copy_val(void *d, const void *s, int64_t n) {
	if (n == 8)
		memcpy(d, s, 8);
	else
		memcpy(d, s, (size_t)n);
}

static inline uint32_t key_hash(const ey_type *kt, const void *k) {
	uint64_t h;
	if (kt->kind == EY_STRING)
		h = ey_str_hash(*(const ey_v *)k);
	else
		h = ey_hash(kt, k);
	return (uint32_t)(h ^ (h >> 32));
}

static inline bool key_eq(const ey_type *kt, const void *a, const void *b) {
	switch (kt->kind) {
	case EY_STRING: {
		ey_v x = *(const ey_v *)a, y = *(const ey_v *)b;
		return x == y || ey_str_eq(x, y);
	}
	case EY_INT:
		return ey_int_eq(*(const ey_v *)a, *(const ey_v *)b);
	}
	return ey_eq(kt, a, b);
}

/* The index is a table of icap slots, each 0 or a key's hash (32 bits)
 * above its position plus one, so that a probe compares keys only when
 * their hashes match and growing the table hashes nothing again. */
#define SLOT(h, i) ((uint64_t)(h) << 32 | (uint64_t)((i) + 1))
#define SLOT_HASH(e) ((uint32_t)((e) >> 32))
#define SLOT_POS(e) ((int64_t)(uint32_t)(e) - 1)

static void index_alloc(ey_map *m, int64_t keys) {
	int64_t icap = 8;
	while (icap < keys * 2 + 2) icap *= 2;
	m->index = calloc((size_t)icap, sizeof(uint64_t));
	if (!m->index) ey_out_of_memory();
	m->icap = icap;
}

static inline void index_put(ey_map *m, uint64_t e) {
	uint64_t mask = (uint64_t)m->icap - 1, j = SLOT_HASH(e) & mask;
	while (m->index[j]) j = (j + 1) & mask;
	m->index[j] = e;
}

/* index_from indexes m's keys from another index of the same keys, room
 * for keys keys */
static void index_from(ey_map *m, const uint64_t *from, int64_t n, int64_t keys) {
	index_alloc(m, keys);
	for (int64_t j = 0; j < n; j++)
		if (from[j]) index_put(m, from[j]);
}

/* index_build indexes m's keys afresh */
static void index_build(ey_map *m) {
	free(m->index);
	index_alloc(m, m->len);
	const ey_type *kt = m->h.ty->args[0];
	int64_t ks = KSIZE(m);
	for (int64_t i = 0; i < m->len; i++) index_put(m, SLOT(key_hash(kt, ITEM(m->keys, i, ks)), i));
}

/* map_find gives the key's position, or -1, and its hash when it is
 * missing. A key that is the same word as the one the last lookup found
 * (the same string, or the same whole number) is found without hashing:
 * `m.insert(k, (m[k] ?? 0) + 1)` looks k up once. */
static inline int64_t map_find(ey_map *m, const void *key, uint32_t *hash) {
	const ey_type *kt = m->h.ty->args[0];
	int64_t last = m->last;
	if ((kt->kind == EY_STRING || kt->kind == EY_INT) && last >= 0 && last < m->len &&
	    ((const ey_v *)m->keys)[last] == *(const ey_v *)key)
		return last;
	uint32_t h = key_hash(kt, key);
	*hash = h;
	if (m->len == 0) return -1;
	int64_t ks = KSIZE(m);
	uint64_t mask = (uint64_t)m->icap - 1, j = h & mask;
	for (;;) {
		uint64_t e = m->index[j];
		if (e == 0) return -1;
		if (SLOT_HASH(e) == h && key_eq(kt, ITEM(m->keys, SLOT_POS(e), ks), key)) {
			m->last = SLOT_POS(e);
			return m->last;
		}
		j = (j + 1) & mask;
	}
}

int64_t ey_map_find(ey_map *m, const void *key) {
	uint32_t h;
	return map_find(m, key, &h);
}

static ey_map *map_alloc(const ey_type *ty, int64_t cap) {
	ey_map *m = ey_alloc(sizeof(ey_map), ty);
	m->len = 0;
	m->cap = cap ? cap : 1;
	m->keys = xmalloc((size_t)(m->cap * KSIZE(m)));
	m->vals = ty->kind == EY_MAP ? xmalloc((size_t)(m->cap * VSIZE(m))) : NULL;
	m->last = -1;
	index_alloc(m, cap);
	return m;
}

/* append adds a key (and value) m doesn't have, with its hash, taking
 * them, and gives where they went */
static int64_t map_append(ey_map *m, uint32_t h, const void *key, const void *val) {
	int64_t ks = KSIZE(m), i = m->len;
	if (i == m->cap) {
		m->cap = grow(m->cap);
		m->keys = xrealloc(m->keys, (size_t)(m->cap * ks));
		if (m->vals) m->vals = xrealloc(m->vals, (size_t)(m->cap * VSIZE(m)));
	}
	if ((i + 1) * 2 + 2 > m->icap) {
		uint64_t *old = m->index;
		index_from(m, old, m->icap, i + 1);
		free(old);
	}
	index_put(m, SLOT(h, i));
	copy_val(ITEM(m->keys, i, ks), key, ks);
	if (m->vals) copy_val(ITEM(m->vals, i, VSIZE(m)), val, VSIZE(m));
	m->len = i + 1;
	return i;
}

/* map_put adds the key (and value) given, taking them, or for a key it
 * has, replaces its value and drops the key given. */
static void map_put(ey_map *m, const void *key, const void *val) {
	const ey_type *ty = m->h.ty;
	uint32_t h;
	int64_t i = map_find(m, key, &h);
	if (i < 0) {
		map_append(m, h, key, val);
		return;
	}
	ey_drop_at(ty->args[0], (void *)key);
	if (m->vals) {
		int64_t vs = VSIZE(m);
		ey_drop_at(ty->args[1], ITEM(m->vals, i, vs));
		copy_val(ITEM(m->vals, i, vs), val, vs);
	}
}

/* put_dup adds copies of the key and value given, which it borrows */
static void put_dup(ey_map *m, const void *key, const void *val) {
	const ey_type *ty = m->h.ty;
	uint32_t h;
	int64_t i = map_find(m, key, &h);
	if (i < 0) {
		i = map_append(m, h, key, val);
		ey_dup_at(ty->args[0], ITEM(m->keys, i, KSIZE(m)));
		if (m->vals) ey_dup_at(ty->args[1], ITEM(m->vals, i, VSIZE(m)));
		return;
	}
	if (m->vals) {
		int64_t vs = VSIZE(m);
		ey_dup_at(ty->args[1], (void *)val);
		ey_drop_at(ty->args[1], ITEM(m->vals, i, vs));
		copy_val(ITEM(m->vals, i, vs), val, vs);
	}
}

ey_v ey_set_new(const ey_type *ty, int64_t n, const void *items) {
	ey_map *m = map_alloc(ty, n);
	int64_t ks = KSIZE(m);
	for (int64_t i = 0; i < n; i++) put_dup(m, ITEM((const char *)items, i, ks), NULL);
	return EY_V(m);
}

ey_v ey_map_new(const ey_type *ty, int64_t n, const void *keys, const void *vals) {
	ey_map *m = map_alloc(ty, n);
	int64_t ks = KSIZE(m), vs = VSIZE(m);
	for (int64_t i = 0; i < n; i++) put_dup(m, ITEM((const char *)keys, i, ks), ITEM((const char *)vals, i, vs));
	return EY_V(m);
}

ey_v ey_set_from(const ey_type *ty, int64_t n, void *const *items) {
	ey_map *m = map_alloc(ty, n);
	for (int64_t i = 0; i < n; i++) map_put(m, items[i], NULL);
	return EY_V(m);
}

ey_v ey_map_from(const ey_type *ty, int64_t n, void *const *keys, void *const *vals) {
	ey_map *m = map_alloc(ty, n);
	for (int64_t i = 0; i < n; i++) map_put(m, keys[i], vals[i]);
	return EY_V(m);
}

/* map_copy copies a set or map, with room for extra more */
static ey_map *map_copy(ey_map *src, int64_t extra) {
	const ey_type *ty = src->h.ty;
	ey_map *m = ey_alloc(sizeof(ey_map), ty);
	int64_t n = src->len, ks = KSIZE(src);
	m->len = n;
	m->cap = n + extra ? n + extra : 1;
	m->keys = xmalloc((size_t)(m->cap * ks));
	memcpy(m->keys, src->keys, (size_t)(n * ks));
	ey_dup_n(ty->args[0], m->keys, n);
	m->vals = NULL;
	if (src->vals) {
		int64_t vs = VSIZE(src);
		m->vals = xmalloc((size_t)(m->cap * vs));
		memcpy(m->vals, src->vals, (size_t)(n * vs));
		ey_dup_n(ty->args[1], m->vals, n);
	}
	m->last = -1;
	index_from(m, src->index, src->icap, n + extra);
	return m;
}

int64_t ey_coll_size(ey_v c) {
	ey_obj *o = EY_OBJ(c);
	switch (o->ty->kind) {
	case EY_LIST:
		return ((ey_list *)o)->len;
	case EY_SET:
	case EY_MAP:
		return ((ey_map *)o)->len;
	case EY_BYTES:
		return ((ey_str *)o)->len;
	case EY_STRING:
		return EY_SMALL_VAL(eyb_str_size(c));
	}
	return 0;
}

/* ---------------------------------------------------------------- the collection functions */

/* items gives the elements of a List or a Set, and their type */
static char *items(ey_v c, int64_t *n, const ey_type **et) {
	ey_obj *o = EY_OBJ(c);
	*et = o->ty->args[0];
	if (o->ty->kind == EY_LIST) {
		*n = ((ey_list *)o)->len;
		return ((ey_list *)o)->items;
	}
	*n = ((ey_map *)o)->len;
	return ((ey_map *)o)->keys;
}

/* collect makes a List or Set of type ty from n values laid out in vals,
 * taking them and the array */
static ey_v collect(const ey_type *ty, int64_t n, char *vals) {
	if (ty->kind == EY_SET) {
		ey_map *m = map_alloc(ty, n);
		int64_t es = ty->args[0]->size;
		for (int64_t i = 0; i < n; i++) map_put(m, ITEM(vals, i, es), NULL);
		free(vals);
		return EY_V(m);
	}
	return ey_list_take(ty, n, vals);
}

ey_v eyb_list_add(ey_v l, const void *x) {
	ey_list *a = EY_LIST(l);
	ey_list *r = list_alloc(a->h.ty, a->len + 1);
	int64_t es = ESIZE(a);
	if (a->len > 0) memcpy(r->items, a->items, (size_t)(a->len * es));
	memcpy(ITEM(r->items, a->len, es), x, (size_t)es);
	r->len = a->len + 1;
	ey_dup_n(a->h.ty->args[0], r->items, r->len);
	return EY_V(r);
}

ey_v eyb_list_concat(ey_v x, ey_v y) {
	ey_list *a = EY_LIST(x), *b = EY_LIST(y);
	ey_list *r = list_alloc(a->h.ty, a->len + b->len);
	int64_t es = ESIZE(a);
	if (a->len > 0) memcpy(r->items, a->items, (size_t)(a->len * es));
	if (b->len > 0) memcpy(ITEM(r->items, a->len, es), b->items, (size_t)(b->len * es));
	r->len = a->len + b->len;
	ey_dup_n(a->h.ty->args[0], r->items, r->len);
	return EY_V(r);
}

ey_v eyb_set_add(ey_v s, const void *x) {
	ey_map *m = map_copy(EY_MAP(s), 1);
	put_dup(m, x, NULL);
	return EY_V(m);
}

ey_v eyb_set_union(ey_v x, ey_v y) {
	ey_map *b = EY_MAP(y);
	ey_map *m = map_copy(EY_MAP(x), b->len);
	int64_t ks = KSIZE(b);
	for (int64_t i = 0; i < b->len; i++) put_dup(m, ITEM(b->keys, i, ks), NULL);
	return EY_V(m);
}

ey_v eyb_map_insert(ey_v mv, const void *k, const void *v) {
	ey_map *m = map_copy(EY_MAP(mv), 1);
	put_dup(m, k, v);
	return EY_V(m);
}

/* ---------------------------------------------------------------- updates in place */

static bool alone(ey_v v) { return EY_OBJ(v)->rc == 1; }

ey_v eyo_list_add(ey_v l, const void *x) {
	if (!alone(l)) {
		ey_v r = eyb_list_add(l, x);
		ey_drop(l);
		return r;
	}
	ey_list *a = EY_LIST(l);
	list_reserve(a, a->len + 1);
	int64_t es = ESIZE(a);
	void *slot = ITEM(a->items, a->len, es);
	memcpy(slot, x, (size_t)es);
	ey_dup_at(a->h.ty->args[0], slot);
	a->len++;
	return l;
}

ey_v eyo_list_concat(ey_v l, ey_v other) {
	if (!alone(l) || l == other) {
		ey_v r = eyb_list_concat(l, other);
		ey_drop(l);
		return r;
	}
	ey_list *a = EY_LIST(l), *b = EY_LIST(other);
	list_reserve(a, a->len + b->len);
	int64_t es = ESIZE(a);
	void *at = ITEM(a->items, a->len, es);
	if (b->len > 0) memcpy(at, b->items, (size_t)(b->len * es));
	ey_dup_n(a->h.ty->args[0], at, b->len);
	a->len += b->len;
	return l;
}

ey_v eyo_set_add(ey_v s, const void *x) {
	if (!alone(s)) {
		ey_v r = eyb_set_add(s, x);
		ey_drop(s);
		return r;
	}
	put_dup(EY_MAP(s), x, NULL);
	return s;
}

ey_v eyo_map_insert(ey_v m, const void *k, const void *v) {
	if (!alone(m)) {
		ey_v r = eyb_map_insert(m, k, v);
		ey_drop(m);
		return r;
	}
	put_dup(EY_MAP(m), k, v);
	return m;
}

ey_v eyo_set_union(ey_v s, ey_v other) {
	if (!alone(s) || s == other) {
		ey_v r = eyb_set_union(s, other);
		ey_drop(s);
		return r;
	}
	ey_map *b = EY_MAP(other);
	int64_t ks = KSIZE(b);
	for (int64_t i = 0; i < b->len; i++) put_dup(EY_MAP(s), ITEM(b->keys, i, ks), NULL);
	return s;
}

ey_v eyb_map_remove(ey_v mv, const void *k) {
	ey_map *src = EY_MAP(mv);
	ey_map *m = map_alloc(src->h.ty, src->len);
	const ey_type *kt = src->h.ty->args[0];
	int64_t ks = KSIZE(src);
	for (int64_t i = 0; i < src->len; i++)
		if (!key_eq(kt, ITEM(src->keys, i, ks), k))
			put_dup(m, ITEM(src->keys, i, ks), src->vals ? ITEM(src->vals, i, VSIZE(src)) : NULL);
	return EY_V(m);
}

ey_v eyo_map_remove(ey_v mv, const void *k) {
	if (!alone(mv)) {
		ey_v r = eyb_map_remove(mv, k);
		ey_drop(mv);
		return r;
	}
	ey_map *m = EY_MAP(mv);
	int64_t i = ey_map_find(m, k);
	if (i < 0) return mv;
	const ey_type *ty = m->h.ty;
	int64_t ks = KSIZE(m);
	ey_drop_at(ty->args[0], ITEM(m->keys, i, ks));
	memmove(ITEM(m->keys, i, ks), ITEM(m->keys, i + 1, ks), (size_t)((m->len - i - 1) * ks));
	if (m->vals) {
		int64_t vs = VSIZE(m);
		ey_drop_at(ty->args[1], ITEM(m->vals, i, vs));
		memmove(ITEM(m->vals, i, vs), ITEM(m->vals, i + 1, vs), (size_t)((m->len - i - 1) * vs));
	}
	m->len--;
	index_build(m);
	return mv;
}

bool eyb_map_contains(ey_v m, const void *k) { return ey_map_find(EY_MAP(m), k) >= 0; }

bool eyb_list_contains(ey_v l, const void *x) {
	int64_t n;
	const ey_type *et;
	char *it = items(l, &n, &et);
	for (int64_t i = 0; i < n; i++)
		if (ey_eq(et, ITEM(it, i, (int64_t)et->size), x)) return true;
	return false;
}

/* some_dup writes Some of a copy of the element at p, which it dups */
static void some_dup(const ey_type *opt, void *out, const ey_type *et, const void *p) {
	ey_dup_at(et, (void *)p);
	ey_some(opt, out, p);
}


void eyb_list_at(const char *at, ey_v l, ey_v i, void *out) {
	ey_list *a = EY_LIST(l);
	int64_t k = EY_IS_SMALL(i) ? EY_SMALL_VAL(i) : -1;
	if (k < 0 || k >= a->len) ey_fault(at, "index out of range");
	int64_t es = ESIZE(a);
	memcpy(out, ITEM(a->items, k, es), (size_t)es);
	ey_dup_at(a->h.ty->args[0], out);
}

ey_v eyb_list_slice(ey_v l, ey_v start, ey_v end, uint32_t flags) {
	ey_list *a = EY_LIST(l);
	int64_t lo, hi;
	eyb_bounds(start, end, flags, a->len, &lo, &hi);
	return ey_list_new(a->h.ty, hi - lo, ITEM(a->items, lo, ESIZE(a)));
}

void eyb_seq_end(ey_v c, bool last, void *out, const ey_type *opt) {
	int64_t n;
	const ey_type *et;
	char *it = items(c, &n, &et);
	if (n == 0) {
		ey_none(opt, out);
		return;
	}
	some_dup(opt, out, et, ITEM(it, last ? n - 1 : 0, (int64_t)et->size));
}

/* call1 calls f on the element at x and writes its result where out points */
static void call1(ey_v f, void *out, const void *x) { ey_call(f, out, (void *const[]){(void *)x}); }

static bool test1(ey_v f, const void *x) {
	uint8_t r = 0;
	call1(f, &r, x);
	return r != 0;
}

ey_v eyb_seq_map(ey_v c, ey_v f, const ey_type *rt) {
	int64_t n;
	const ey_type *et;
	char *it = items(c, &n, &et);
	int64_t os = rt->args[0]->size;
	char *out = xmalloc((size_t)(n * os));
	for (int64_t i = 0; i < n; i++) call1(f, ITEM(out, i, os), ITEM(it, i, (int64_t)et->size));
	return collect(rt, n, out);
}

ey_v eyb_seq_filter(ey_v c, ey_v f, const ey_type *rt) {
	int64_t n, k = 0;
	const ey_type *et;
	char *it = items(c, &n, &et);
	int64_t es = et->size;
	char *out = xmalloc((size_t)(n * es));
	for (int64_t i = 0; i < n; i++) {
		if (test1(f, ITEM(it, i, es))) {
			memcpy(ITEM(out, k, es), ITEM(it, i, es), (size_t)es);
			ey_dup_at(et, ITEM(out, k, es));
			k++;
		}
	}
	return collect(rt, k, out);
}

void eyb_seq_find(ey_v c, ey_v f, void *out, const ey_type *opt) {
	int64_t n;
	const ey_type *et;
	char *it = items(c, &n, &et);
	for (int64_t i = 0; i < n; i++) {
		if (test1(f, ITEM(it, i, (int64_t)et->size))) {
			some_dup(opt, out, et, ITEM(it, i, (int64_t)et->size));
			return;
		}
	}
	ey_none(opt, out);
}

/* how: 0 any, 1 all, 2 count */
ey_v eyb_seq_test(ey_v c, ey_v f, int how) {
	int64_t n, k = 0;
	const ey_type *et;
	char *it = items(c, &n, &et);
	for (int64_t i = 0; i < n; i++) {
		bool yes = test1(f, ITEM(it, i, (int64_t)et->size));
		if (how == 0 && yes) return 1;
		if (how == 1 && !yes) return 0;
		k += yes;
	}
	if (how == 2) return EY_SMALL(k);
	return how == 1;
}

void eyb_seq_reduce(ey_v c, const void *init, ey_v f, void *out, const ey_type *acct) {
	int64_t n;
	const ey_type *et;
	char *it = items(c, &n, &et);
	size_t as = acct->size;
	char acc[64], next[64];
	char *a = as <= 64 ? acc : xmalloc(as), *nx = as <= 64 ? next : xmalloc(as);
	memcpy(a, init, as);
	ey_dup_at(acct, a);
	for (int64_t i = 0; i < n; i++) {
		ey_call(f, nx, (void *const[]){a, ITEM(it, i, (int64_t)et->size)});
		ey_drop_at(acct, a);
		memcpy(a, nx, as);
	}
	memcpy(out, a, as);
	if (a != acc) free(a);
	if (nx != next) free(nx);
}

ey_v eyb_seq_sum(ey_v c) {
	int64_t n;
	const ey_type *et;
	char *it = items(c, &n, &et);
	switch (et->kind) {
	case EY_INT: {
		ey_v acc = EY_SMALL(0);
		for (int64_t i = 0; i < n; i++) {
			ey_v next = ey_int_add(acc, ((ey_v *)it)[i]);
			ey_drop(acc);
			acc = next;
		}
		return acc;
	}
	case EY_FLOAT: {
		double acc = 0;
		for (int64_t i = 0; i < n; i++) acc = i == 0 ? ((double *)it)[i] : acc + ((double *)it)[i];
		return ey_f2v(acc);
	}
	default: {
		uint64_t acc = 0;
		for (int64_t i = 0; i < n; i++) acc += ey_load_v(et, ITEM(it, i, (int64_t)et->size));
		return acc;
	}
	}
}

ey_v eyb_seq_enumerate(ey_v c, const ey_type *rt) {
	int64_t n;
	const ey_type *et;
	char *it = items(c, &n, &et);
	const ey_type *tt = rt->args[0];
	int64_t ts = tt->size, es = et->size;
	char *out = xmalloc((size_t)(n * ts));
	for (int64_t i = 0; i < n; i++) {
		ey_v idx = EY_SMALL(i);
		ey_dup_at(et, ITEM(it, i, es));
		ey_tuple_make(tt, ITEM(out, i, ts), (const void *const[]){&idx, ITEM(it, i, es)});
	}
	return ey_list_take(rt, n, out);
}

ey_v eyb_seq_take_drop(const char *at, ey_v c, ey_v count, bool take, const ey_type *rt) {
	int64_t n, k;
	if (!ey_int_i64_of(count, &k) || k < 0) ey_fault(at, "a count is an Int that isn't negative");
	const ey_type *et;
	char *it = items(c, &n, &et);
	if (k > n) k = n;
	int64_t es = et->size;
	if (rt->kind == EY_SET) return take ? ey_set_new(rt, k, it) : ey_set_new(rt, n - k, ITEM(it, k, es));
	return take ? ey_list_new(rt, k, it) : ey_list_new(rt, n - k, ITEM(it, k, es));
}

ey_v eyb_seq_reverse(ey_v c, const ey_type *rt) {
	int64_t n;
	const ey_type *et;
	char *it = items(c, &n, &et);
	int64_t es = et->size;
	char *out = xmalloc((size_t)(n * es));
	for (int64_t i = 0; i < n; i++) memcpy(ITEM(out, i, es), ITEM(it, n - 1 - i, es), (size_t)es);
	ey_dup_n(et, out, n);
	return ey_list_take(rt, n, out);
}

/* sort_small sorts whole numbers that are all small, whose words order as
 * their values do: insertion sort for short runs, merge sort above. */
static void sort_small(int64_t *a, int64_t *tmp, int64_t n) {
	if (n <= 24) {
		for (int64_t i = 1; i < n; i++) {
			int64_t x = a[i], j = i;
			for (; j > 0 && a[j - 1] > x; j--) a[j] = a[j - 1];
			a[j] = x;
		}
		return;
	}
	int64_t m = n / 2;
	sort_small(a, tmp, m);
	sort_small(a + m, tmp, n - m);
	if (a[m - 1] <= a[m]) return;
	memcpy(tmp, a, (size_t)m * sizeof(int64_t));
	int64_t i = 0, j = m, k = 0;
	while (i < m && j < n) a[k++] = a[j] < tmp[i] ? a[j++] : tmp[i++];
	while (i < m) a[k++] = tmp[i++];
}

static void merge_sort(const ey_type *t, char *a, char *tmp, int64_t n) {
	if (n < 2) return;
	int64_t es = t->size, m = n / 2;
	merge_sort(t, a, tmp, m);
	merge_sort(t, ITEM(a, m, es), tmp, n - m);
	if (ey_cmp(t, ITEM(a, m, es), ITEM(a, m - 1, es)) >= 0) return;
	memcpy(tmp, a, (size_t)(m * es));
	int64_t i = 0, j = m, k = 0;
	while (i < m && j < n) {
		if (ey_cmp(t, ITEM(a, j, es), ITEM(tmp, i, es)) < 0)
			memcpy(ITEM(a, k++, es), ITEM(a, j++, es), (size_t)es);
		else
			memcpy(ITEM(a, k++, es), ITEM(tmp, i++, es), (size_t)es);
	}
	while (i < m) memcpy(ITEM(a, k++, es), ITEM(tmp, i++, es), (size_t)es);
}

static bool all_small(const ey_v *a, int64_t n) {
	for (int64_t i = 0; i < n; i++)
		if (!EY_IS_SMALL(a[i])) return false;
	return true;
}

ey_v eyb_seq_sort(ey_v c, const ey_type *rt) {
	int64_t n;
	const ey_type *et;
	char *it = items(c, &n, &et);
	ey_v r = ey_list_new(rt, n, it);
	char *a = EY_LIST(r)->items;
	char *tmp = xmalloc((size_t)((n / 2 + 1) * (int64_t)et->size));
	if (et->kind == EY_INT && all_small((const ey_v *)a, n))
		sort_small((int64_t *)a, (int64_t *)tmp, n);
	else
		merge_sort(rt->args[0], a, tmp, n);
	free(tmp);
	return r;
}
/* which: 0 keys, 1 values, 2 entries */
ey_v eyb_map_list(ey_v mv, int which, const ey_type *rt) {
	ey_map *m = EY_MAP(mv);
	if (which == 0) return ey_list_new(rt, m->len, m->keys);
	if (which == 1) return ey_list_new(rt, m->len, m->vals);
	const ey_type *tt = rt->args[0];
	int64_t ts = tt->size, ks = KSIZE(m), vs = VSIZE(m);
	char *out = xmalloc((size_t)(m->len * ts));
	for (int64_t i = 0; i < m->len; i++) {
		ey_dup_at(m->h.ty->args[0], ITEM(m->keys, i, ks));
		ey_dup_at(m->h.ty->args[1], ITEM(m->vals, i, vs));
		ey_tuple_make(tt, ITEM(out, i, ts), (const void *const[]){ITEM(m->keys, i, ks), ITEM(m->vals, i, vs)});
	}
	return ey_list_take(rt, m->len, out);
}

ey_v eyb_seq_convert(ey_v c, const ey_type *rt) {
	int64_t n;
	const ey_type *et;
	char *it = items(c, &n, &et);
	if (rt->kind == EY_SET) return ey_set_new(rt, n, it);
	return ey_list_new(rt, n, it);
}

ey_v eyb_seq_join(ey_v c, ey_v sep) {
	int64_t n;
	const ey_type *et;
	char *it = items(c, &n, &et);
	ey_buf b = {0};
	for (int64_t i = 0; i < n; i++) {
		if (i > 0 && sep) ey_buf_add(&b, EY_STR(sep)->b, EY_STR(sep)->len);
		ey_show_to(&b, et, ITEM(it, i, (int64_t)et->size), false);
	}
	return ey_buf_take(&b);
}

ey_v eyb_seq_flat_map(ey_v c, ey_v f, const ey_type *rt) {
	int64_t n;
	const ey_type *et;
	char *it = items(c, &n, &et);
	ey_list *r = list_alloc(rt, n);
	for (int64_t i = 0; i < n; i++) {
		ey_v part = 0;
		call1(f, &part, ITEM(it, i, (int64_t)et->size));
		int64_t pn;
		const ey_type *pt;
		char *pi = items(part, &pn, &pt);
		list_reserve(r, r->len + pn);
		void *at = ITEM(r->items, r->len, (int64_t)pt->size);
		if (pn > 0) memcpy(at, pi, (size_t)(pn * (int64_t)pt->size));
		ey_dup_n(pt, at, pn);
		r->len += pn;
		ey_drop(part);
	}
	return EY_V(r);
}

ey_v eyb_seq_zip(ey_v x, ey_v y, const ey_type *rt) {
	int64_t an, bn;
	const ey_type *at, *bt;
	char *a = items(x, &an, &at), *b = items(y, &bn, &bt);
	int64_t n = an < bn ? an : bn;
	const ey_type *tt = rt->args[0];
	int64_t ts = tt->size;
	char *out = xmalloc((size_t)(n * ts));
	for (int64_t i = 0; i < n; i++) {
		void *ai = ITEM(a, i, (int64_t)at->size), *bi = ITEM(b, i, (int64_t)bt->size);
		ey_dup_at(at, ai);
		ey_dup_at(bt, bi);
		ey_tuple_make(tt, ITEM(out, i, ts), (const void *const[]){ai, bi});
	}
	return ey_list_take(rt, n, out);
}

/* range_list is the Ints of a range with both ends */
ey_v eyb_range_list(ey_v r, const ey_type *rt) { return ey_lazy_list(r, rt); }

/* mean is the Float nearest to total / n, as the interpreter's big.Rat
 * gives it: exact operands are divided once when they fit in a double;
 * otherwise the quotient is taken to at least 70 bits, with a bit below
 * them for any remainder, and rounded once. */
static double mean(ey_v total, int64_t n) {
	int64_t t;
	if (ey_int_i64_of(total, &t) && t > -(INT64_C(1) << 53) && t < (INT64_C(1) << 53) && n < (INT64_C(1) << 53))
		return (double)t / (double)n;
	bool neg = ey_int_cmp(total, EY_SMALL(0)) < 0;
	ey_v s = neg ? ey_int_neg(total) : (ey_dup(total), total);
	int e;
	frexp(ey_int_to_f64(s), &e);
	int k = 72 + 64 - e;
	if (k < 0) k = 0;
	for (int left = k; left > 0;) {
		int step = left > 30 ? 30 : left;
		ey_v next = ey_int_mul(s, EY_SMALL(INT64_C(1) << step));
		ey_drop(s);
		s = next;
		left -= step;
	}
	ey_v d = ey_int_i64(n);
	ey_v q = ey_int_div("<average>", s, d);
	ey_v r = ey_int_mod("<average>", s, d);
	if (r != EY_SMALL(0)) {
		ey_v q2 = ey_int_mul(q, EY_SMALL(2));
		ey_v q3 = ey_int_add(q2, EY_SMALL(1));
		ey_drop(q);
		ey_drop(q2);
		q = q3;
		k++;
	}
	double m = ldexp(ey_int_to_f64(q), -k);
	ey_drop(s);
	ey_drop(q);
	ey_drop(r);
	ey_drop(d);
	return neg ? -m : m;
}

void eyb_seq_average(ey_v c, void *out, const ey_type *opt) {
	int64_t n;
	const ey_type *et;
	char *it = items(c, &n, &et);
	if (n == 0) {
		ey_none(opt, out);
		return;
	}
	if (et->kind == EY_FLOAT) {
		double sum = 0;
		for (int64_t i = 0; i < n; i++) sum += ((double *)it)[i];
		double m = sum / (double)n;
		ey_some(opt, out, &m);
		return;
	}
	ey_v total = EY_SMALL(0);
	for (int64_t i = 0; i < n; i++) {
		ey_v next = ey_int_add(total, ((ey_v *)it)[i]);
		ey_drop(total);
		total = next;
	}
	double m = mean(total, n);
	ey_drop(total);
	ey_some(opt, out, &m);
}

/* ---------------------------------------------------------------- elements and bytes */

void *eyo_list_slot(const char *at, ey_v *p, ey_v i) {
	ey_list *a = EY_LIST(*p);
	int64_t k;
	if (!ey_int_i64_of(i, &k) || k < 0 || k >= a->len) {
		ey_buf b = {0};
		ey_int_write(&b, i);
		ey_buf_char(&b, 0);
		ey_faultf(at, "index %s is out of range for a list of %lld", b.b, (long long)a->len);
	}
	if (!alone(*p)) {
		ey_v c = ey_list_new(a->h.ty, a->len, a->items);
		ey_drop(*p);
		*p = c;
		a = EY_LIST(c);
	}
	return ITEM(a->items, k, ESIZE(a));
}

ey_v eyb_list_to_bytes(ey_v l) {
	ey_list *a = EY_LIST(l);
	ey_str *s = ey_alloc(sizeof(ey_str) + (size_t)a->len + 1, &ey_t_bytes);
	s->len = a->len;
	s->cap = 0;
	if (a->len > 0) memcpy(s->b, a->items, (size_t)a->len);
	s->b[a->len] = 0;
	return EY_V(s);
}
