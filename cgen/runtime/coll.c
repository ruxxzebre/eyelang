/* Lists, sets and maps, iteration, and the collection functions. Sets and
 * maps keep their elements in insertion order, as the interpreter's do, with
 * a hash index beside them. Every function leaves the collections it is
 * given as they were: an update makes a new one. */
#include <math.h>
#include <stdio.h>
#include <stdlib.h>

#include "eye.h"
#include "builtins.h"

static void *xmalloc(size_t n) {
	void *p = malloc(n ? n : 1);
	if (!p) {
		fputs("out of memory\n", stderr);
		exit(2);
	}
	return p;
}

/* ---------------------------------------------------------------- lists */

static ey_list *list_alloc(const ey_type *ty, int64_t cap) {
	ey_list *l = ey_alloc(sizeof(ey_list), ty);
	l->len = 0;
	l->cap = cap;
	l->items = xmalloc(sizeof(ey_v) * (size_t)cap);
	return l;
}

ey_v ey_list_take(const ey_type *ty, int64_t n, ey_v *items) {
	ey_list *l = ey_alloc(sizeof(ey_list), ty);
	l->len = l->cap = n;
	l->items = items ? items : xmalloc(sizeof(ey_v));
	return EY_V(l);
}

ey_v ey_list_new(const ey_type *ty, int64_t n, const ey_v *items) {
	ey_list *l = list_alloc(ty, n);
	bool counted = ey_counted(ty->args[0]);
	for (int64_t i = 0; i < n; i++) {
		if (counted) ey_dup(items[i]);
		l->items[i] = items[i];
	}
	l->len = n;
	return EY_V(l);
}

static void list_push(ey_list *l, ey_v v) {
	if (l->len == l->cap) {
		l->cap = l->cap ? l->cap * 2 : 4;
		l->items = realloc(l->items, sizeof(ey_v) * (size_t)l->cap);
		if (!l->items) {
			fputs("out of memory\n", stderr);
			exit(2);
		}
	}
	l->items[l->len++] = v;
}

/* ---------------------------------------------------------------- sets and maps */

static void index_build(ey_map *m) {
	int64_t icap = 8;
	while (icap < m->len * 2 + 2) icap *= 2;
	free(m->index);
	m->index = calloc((size_t)icap, sizeof(int32_t));
	m->icap = icap;
	const ey_type *kt = m->h.ty->args[0];
	for (int64_t i = 0; i < m->len; i++) {
		uint64_t h = ey_hash(kt, m->keys[i]) & (uint64_t)(icap - 1);
		while (m->index[h]) h = (h + 1) & (uint64_t)(icap - 1);
		m->index[h] = (int32_t)(i + 1);
	}
}

int64_t ey_map_find(ey_map *m, ey_v key) {
	if (m->len == 0) return -1;
	const ey_type *kt = m->h.ty->args[0];
	uint64_t h = ey_hash(kt, key) & (uint64_t)(m->icap - 1);
	for (;;) {
		int32_t i = m->index[h];
		if (i == 0) return -1;
		if (ey_eq(kt, m->keys[i - 1], key)) return i - 1;
		h = (h + 1) & (uint64_t)(m->icap - 1);
	}
}

static ey_map *map_alloc(const ey_type *ty, int64_t cap) {
	ey_map *m = ey_alloc(sizeof(ey_map), ty);
	m->len = 0;
	m->cap = cap ? cap : 1;
	m->keys = xmalloc(sizeof(ey_v) * (size_t)m->cap);
	m->vals = ty->kind == EY_MAP ? xmalloc(sizeof(ey_v) * (size_t)m->cap) : NULL;
	m->index = NULL;
	m->icap = 0;
	index_build(m);
	return m;
}

/* map_put adds key (and val), taking them, or for a key it has, replaces
 * its value and drops the key given. */
static void map_put(ey_map *m, ey_v key, ey_v val) {
	const ey_type *ty = m->h.ty;
	int64_t i = ey_map_find(m, key);
	if (i >= 0) {
		if (ey_counted(ty->args[0])) ey_drop(key);
		if (m->vals) {
			if (ey_counted(ty->args[1])) ey_drop(m->vals[i]);
			m->vals[i] = val;
		}
		return;
	}
	if (m->len == m->cap) {
		m->cap *= 2;
		m->keys = realloc(m->keys, sizeof(ey_v) * (size_t)m->cap);
		if (m->vals) m->vals = realloc(m->vals, sizeof(ey_v) * (size_t)m->cap);
	}
	m->keys[m->len] = key;
	if (m->vals) m->vals[m->len] = val;
	m->len++;
	if (m->len * 2 + 2 > m->icap) {
		index_build(m);
	} else {
		uint64_t h = ey_hash(ty->args[0], key) & (uint64_t)(m->icap - 1);
		while (m->index[h]) h = (h + 1) & (uint64_t)(m->icap - 1);
		m->index[h] = (int32_t)m->len;
	}
}

static void put_dup(ey_map *m, ey_v key, ey_v val) {
	const ey_type *ty = m->h.ty;
	if (ey_counted(ty->args[0])) ey_dup(key);
	if (m->vals && ey_counted(ty->args[1])) ey_dup(val);
	map_put(m, key, val);
}

ey_v ey_set_new(const ey_type *ty, int64_t n, const ey_v *items) {
	ey_map *m = map_alloc(ty, n);
	for (int64_t i = 0; i < n; i++) put_dup(m, items[i], 0);
	return EY_V(m);
}

ey_v ey_map_new(const ey_type *ty, int64_t n, const ey_v *keys, const ey_v *vals) {
	ey_map *m = map_alloc(ty, n);
	for (int64_t i = 0; i < n; i++) put_dup(m, keys[i], vals[i]);
	return EY_V(m);
}

static ey_map *map_copy(ey_map *src, int64_t extra) {
	ey_map *m = map_alloc(src->h.ty, src->len + extra);
	for (int64_t i = 0; i < src->len; i++) put_dup(m, src->keys[i], src->vals ? src->vals[i] : 0);
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

/* items gives the elements of a List or a Set */
static const ey_v *items(ey_v c, int64_t *n) {
	ey_obj *o = EY_OBJ(c);
	if (o->ty->kind == EY_LIST) {
		*n = ((ey_list *)o)->len;
		return ((ey_list *)o)->items;
	}
	*n = ((ey_map *)o)->len;
	return ((ey_map *)o)->keys;
}

static const ey_type *elem_type(ey_v c) { return EY_OBJ(c)->ty->args[0]; }

/* collect makes a List or Set of type ty from n values it takes */
static ey_v collect(const ey_type *ty, int64_t n, ey_v *vals) {
	if (ty->kind == EY_SET) {
		ey_map *m = map_alloc(ty, n);
		for (int64_t i = 0; i < n; i++) map_put(m, vals[i], 0);
		free(vals);
		return EY_V(m);
	}
	return ey_list_take(ty, n, vals);
}

ey_v eyb_list_add(ey_v l, ey_v x) {
	ey_list *a = EY_LIST(l);
	ey_list *r = list_alloc(a->h.ty, a->len + 1);
	for (int64_t i = 0; i < a->len; i++) list_push(r, a->items[i]);
	list_push(r, x);
	if (ey_counted(a->h.ty->args[0]))
		for (int64_t i = 0; i < r->len; i++) ey_dup(r->items[i]);
	return EY_V(r);
}

ey_v eyb_list_concat(ey_v x, ey_v y) {
	ey_list *a = EY_LIST(x), *b = EY_LIST(y);
	ey_list *r = list_alloc(a->h.ty, a->len + b->len);
	for (int64_t i = 0; i < a->len; i++) list_push(r, a->items[i]);
	for (int64_t i = 0; i < b->len; i++) list_push(r, b->items[i]);
	if (ey_counted(a->h.ty->args[0]))
		for (int64_t i = 0; i < r->len; i++) ey_dup(r->items[i]);
	return EY_V(r);
}

ey_v eyb_set_add(ey_v s, ey_v x) {
	ey_map *m = map_copy(EY_MAP(s), 1);
	put_dup(m, x, 0);
	return EY_V(m);
}

ey_v eyb_set_union(ey_v x, ey_v y) {
	ey_map *b = EY_MAP(y);
	ey_map *m = map_copy(EY_MAP(x), b->len);
	for (int64_t i = 0; i < b->len; i++) put_dup(m, b->keys[i], 0);
	return EY_V(m);
}

ey_v eyb_map_insert(ey_v mv, ey_v k, ey_v v) {
	ey_map *m = map_copy(EY_MAP(mv), 1);
	put_dup(m, k, v);
	return EY_V(m);
}

/* ---------------------------------------------------------------- updates in place */

static bool alone(ey_v v) { return EY_OBJ(v)->rc == 1; }

ey_v eyo_list_add(ey_v l, ey_v x) {
	if (!alone(l)) {
		ey_v r = eyb_list_add(l, x);
		ey_drop(l);
		return r;
	}
	ey_list *a = EY_LIST(l);
	if (ey_counted(a->h.ty->args[0])) ey_dup(x);
	list_push(a, x);
	return l;
}

ey_v eyo_list_concat(ey_v l, ey_v other) {
	if (!alone(l) || l == other) {
		ey_v r = eyb_list_concat(l, other);
		ey_drop(l);
		return r;
	}
	ey_list *a = EY_LIST(l), *b = EY_LIST(other);
	bool counted = ey_counted(a->h.ty->args[0]);
	for (int64_t i = 0; i < b->len; i++) {
		if (counted) ey_dup(b->items[i]);
		list_push(a, b->items[i]);
	}
	return l;
}

ey_v eyo_set_add(ey_v s, ey_v x) {
	if (!alone(s)) {
		ey_v r = eyb_set_add(s, x);
		ey_drop(s);
		return r;
	}
	put_dup(EY_MAP(s), x, 0);
	return s;
}

ey_v eyo_map_insert(ey_v m, ey_v k, ey_v v) {
	if (!alone(m)) {
		ey_v r = eyb_map_insert(m, k, v);
		ey_drop(m);
		return r;
	}
	put_dup(EY_MAP(m), k, v);
	return m;
}

ey_v eyb_map_remove(ey_v mv, ey_v k) {
	ey_map *src = EY_MAP(mv);
	ey_map *m = map_alloc(src->h.ty, src->len);
	const ey_type *kt = src->h.ty->args[0];
	for (int64_t i = 0; i < src->len; i++)
		if (!ey_eq(kt, src->keys[i], k)) put_dup(m, src->keys[i], src->vals ? src->vals[i] : 0);
	return EY_V(m);
}

bool eyb_map_contains(ey_v m, ey_v k) { return ey_map_find(EY_MAP(m), k) >= 0; }

bool eyb_list_contains(ey_v l, ey_v x) {
	int64_t n;
	const ey_v *it = items(l, &n);
	for (int64_t i = 0; i < n; i++)
		if (ey_eq(elem_type(l), it[i], x)) return true;
	return false;
}

ey_v eyb_map_get(ey_v mv, ey_v k, const ey_type *opt) {
	ey_map *m = EY_MAP(mv);
	int64_t i = ey_map_find(m, k);
	if (i < 0) return ey_none(opt);
	ey_v v = m->vals[i];
	if (ey_counted(m->h.ty->args[1])) ey_dup(v);
	return ey_some(opt, v);
}

ey_v eyb_list_index(ey_v l, ey_v i, const ey_type *opt) {
	ey_list *a = EY_LIST(l);
	if (!EY_IS_SMALL(i)) return ey_none(opt);
	int64_t k = EY_SMALL_VAL(i);
	if (k < 0 || k >= a->len) return ey_none(opt);
	ey_v v = a->items[k];
	if (ey_counted(a->h.ty->args[0])) ey_dup(v);
	return ey_some(opt, v);
}

ey_v eyb_list_at(const char *at, ey_v l, ey_v i) {
	ey_list *a = EY_LIST(l);
	int64_t k = EY_IS_SMALL(i) ? EY_SMALL_VAL(i) : -1;
	if (k < 0 || k >= a->len) ey_fault(at, "index out of range");
	ey_v v = a->items[k];
	if (ey_counted(a->h.ty->args[0])) ey_dup(v);
	return v;
}

ey_v eyb_list_slice(ey_v l, ey_v start, ey_v end, uint32_t flags) {
	ey_list *a = EY_LIST(l);
	int64_t lo, hi;
	eyb_bounds(start, end, flags, a->len, &lo, &hi);
	return ey_list_new(a->h.ty, hi - lo, a->items + lo);
}

ey_v eyb_seq_end(ey_v c, bool last, const ey_type *opt) {
	int64_t n;
	const ey_v *it = items(c, &n);
	if (n == 0) return ey_none(opt);
	ey_v v = it[last ? n - 1 : 0];
	if (ey_counted(elem_type(c))) ey_dup(v);
	return ey_some(opt, v);
}

ey_v eyb_seq_map(ey_v c, ey_v f, const ey_type *rt) {
	int64_t n;
	const ey_v *it = items(c, &n);
	ey_v *out = xmalloc(sizeof(ey_v) * (size_t)n);
	for (int64_t i = 0; i < n; i++) out[i] = ey_call(f, &it[i]);
	return collect(rt, n, out);
}

ey_v eyb_seq_filter(ey_v c, ey_v f, const ey_type *rt) {
	int64_t n, k = 0;
	const ey_v *it = items(c, &n);
	ey_v *out = xmalloc(sizeof(ey_v) * (size_t)n);
	bool counted = ey_counted(elem_type(c));
	for (int64_t i = 0; i < n; i++) {
		if (ey_call(f, &it[i])) {
			if (counted) ey_dup(it[i]);
			out[k++] = it[i];
		}
	}
	return collect(rt, k, out);
}

ey_v eyb_seq_find(ey_v c, ey_v f, const ey_type *opt) {
	int64_t n;
	const ey_v *it = items(c, &n);
	for (int64_t i = 0; i < n; i++) {
		if (ey_call(f, &it[i])) {
			if (ey_counted(elem_type(c))) ey_dup(it[i]);
			return ey_some(opt, it[i]);
		}
	}
	return ey_none(opt);
}

/* how: 0 any, 1 all, 2 count */
ey_v eyb_seq_test(ey_v c, ey_v f, int how) {
	int64_t n, k = 0;
	const ey_v *it = items(c, &n);
	for (int64_t i = 0; i < n; i++) {
		bool yes = ey_call(f, &it[i]) != 0;
		if (how == 0 && yes) return 1;
		if (how == 1 && !yes) return 0;
		k += yes;
	}
	if (how == 2) return EY_SMALL(k);
	return how == 1;
}

ey_v eyb_seq_reduce(ey_v c, ey_v init, ey_v f, const ey_type *acct) {
	int64_t n;
	const ey_v *it = items(c, &n);
	ey_v acc = init;
	bool counted = ey_counted(acct);
	if (counted) ey_dup(acc);
	for (int64_t i = 0; i < n; i++) {
		ey_v args[2] = {acc, it[i]};
		ey_v next = ey_call(f, args);
		if (counted) ey_drop(acc);
		acc = next;
	}
	return acc;
}

ey_v eyb_seq_sum(ey_v c) {
	int64_t n;
	const ey_v *it = items(c, &n);
	const ey_type *et = elem_type(c);
	switch (et->kind) {
	case EY_INT: {
		ey_v acc = EY_SMALL(0);
		for (int64_t i = 0; i < n; i++) {
			ey_v next = ey_int_add(acc, it[i]);
			ey_drop(acc);
			acc = next;
		}
		return acc;
	}
	case EY_FLOAT: {
		double acc = 0;
		for (int64_t i = 0; i < n; i++) acc = i == 0 ? ey_v2f(it[i]) : acc + ey_v2f(it[i]);
		return ey_f2v(acc);
	}
	default: {
		uint64_t acc = 0;
		for (int64_t i = 0; i < n; i++) acc += it[i];
		return acc;
	}
	}
}

ey_v eyb_seq_enumerate(ey_v c, const ey_type *rt) {
	int64_t n;
	const ey_v *it = items(c, &n);
	const ey_type *tt = rt->args[0];
	bool counted = ey_counted(elem_type(c));
	ey_v *out = xmalloc(sizeof(ey_v) * (size_t)n);
	for (int64_t i = 0; i < n; i++) {
		ey_agg *t = ey_agg_new(tt, 0, 2);
		t->f[0] = EY_SMALL(i);
		t->f[1] = it[i];
		if (counted) ey_dup(it[i]);
		out[i] = EY_V(t);
	}
	return ey_list_take(rt, n, out);
}

ey_v eyb_seq_take_drop(const char *at, ey_v c, ey_v count, bool take, const ey_type *rt) {
	int64_t n, k;
	if (!ey_int_i64_of(count, &k) || k < 0) ey_fault(at, "a count is an Int that isn't negative");
	const ey_v *it = items(c, &n);
	if (k > n) k = n;
	if (rt->kind == EY_SET) return take ? ey_set_new(rt, k, it) : ey_set_new(rt, n - k, it + k);
	return take ? ey_list_new(rt, k, it) : ey_list_new(rt, n - k, it + k);
}

ey_v eyb_seq_reverse(ey_v c, const ey_type *rt) {
	int64_t n;
	const ey_v *it = items(c, &n);
	ey_v *out = xmalloc(sizeof(ey_v) * (size_t)n);
	bool counted = ey_counted(elem_type(c));
	for (int64_t i = 0; i < n; i++) {
		out[i] = it[n - 1 - i];
		if (counted) ey_dup(out[i]);
	}
	return ey_list_take(rt, n, out);
}

static void merge_sort(const ey_type *t, ey_v *a, ey_v *tmp, int64_t n) {
	if (n < 2) return;
	int64_t m = n / 2;
	merge_sort(t, a, tmp, m);
	merge_sort(t, a + m, tmp, n - m);
	int64_t i = 0, j = m, k = 0;
	while (i < m && j < n) tmp[k++] = ey_cmp(t, a[j], a[i]) < 0 ? a[j++] : a[i++];
	while (i < m) tmp[k++] = a[i++];
	while (j < n) tmp[k++] = a[j++];
	memcpy(a, tmp, sizeof(ey_v) * (size_t)n);
}

ey_v eyb_seq_sort(ey_v c, const ey_type *rt) {
	int64_t n;
	const ey_v *it = items(c, &n);
	ey_v r = ey_list_new(rt, n, it);
	ey_v *tmp = xmalloc(sizeof(ey_v) * (size_t)n);
	merge_sort(rt->args[0], EY_LIST(r)->items, tmp, n);
	free(tmp);
	return r;
}

/* which: 0 keys, 1 values, 2 entries */
ey_v eyb_map_list(ey_v mv, int which, const ey_type *rt) {
	ey_map *m = EY_MAP(mv);
	if (which == 0) return ey_list_new(rt, m->len, m->keys);
	if (which == 1) return ey_list_new(rt, m->len, m->vals);
	ey_v *out = xmalloc(sizeof(ey_v) * (size_t)m->len);
	const ey_type *tt = rt->args[0];
	for (int64_t i = 0; i < m->len; i++) {
		ey_agg *t = ey_agg_new(tt, 0, 2);
		t->f[0] = m->keys[i];
		t->f[1] = m->vals[i];
		if (ey_counted(tt->args[0])) ey_dup(t->f[0]);
		if (ey_counted(tt->args[1])) ey_dup(t->f[1]);
		out[i] = EY_V(t);
	}
	return ey_list_take(rt, m->len, out);
}

ey_v eyb_seq_convert(ey_v c, const ey_type *rt) {
	int64_t n;
	const ey_v *it = items(c, &n);
	if (rt->kind == EY_SET) return ey_set_new(rt, n, it);
	return ey_list_new(rt, n, it);
}

ey_v eyb_seq_join(ey_v c, ey_v sep) {
	int64_t n;
	const ey_v *it = items(c, &n);
	const ey_type *et = elem_type(c);
	ey_buf b = {0};
	for (int64_t i = 0; i < n; i++) {
		if (i > 0 && sep) ey_buf_add(&b, EY_STR(sep)->b, EY_STR(sep)->len);
		ey_show_to(&b, et, it[i], false);
	}
	return ey_buf_take(&b);
}

ey_v eyb_seq_flat_map(ey_v c, ey_v f, const ey_type *rt) {
	int64_t n;
	const ey_v *it = items(c, &n);
	ey_list *r = list_alloc(rt, n);
	bool counted = ey_counted(rt->args[0]);
	for (int64_t i = 0; i < n; i++) {
		ey_v part = ey_call(f, &it[i]);
		int64_t pn;
		const ey_v *pi = items(part, &pn);
		for (int64_t j = 0; j < pn; j++) {
			if (counted) ey_dup(pi[j]);
			list_push(r, pi[j]);
		}
		ey_drop(part);
	}
	return EY_V(r);
}

ey_v eyb_seq_zip(ey_v x, ey_v y, const ey_type *rt) {
	int64_t an, bn;
	const ey_v *a = items(x, &an), *b = items(y, &bn);
	int64_t n = an < bn ? an : bn;
	const ey_type *tt = rt->args[0];
	ey_v *out = xmalloc(sizeof(ey_v) * (size_t)n);
	for (int64_t i = 0; i < n; i++) {
		ey_agg *t = ey_agg_new(tt, 0, 2);
		t->f[0] = a[i];
		t->f[1] = b[i];
		if (ey_counted(tt->args[0])) ey_dup(a[i]);
		if (ey_counted(tt->args[1])) ey_dup(b[i]);
		out[i] = EY_V(t);
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

ey_v eyb_seq_average(ey_v c, const ey_type *opt) {
	int64_t n;
	const ey_v *it = items(c, &n);
	if (n == 0) return ey_none(opt);
	if (elem_type(c)->kind == EY_FLOAT) {
		double sum = 0;
		for (int64_t i = 0; i < n; i++) sum += ey_v2f(it[i]);
		return ey_some(opt, ey_f2v(sum / (double)n));
	}
	ey_v total = EY_SMALL(0);
	for (int64_t i = 0; i < n; i++) {
		ey_v next = ey_int_add(total, it[i]);
		ey_drop(total);
		total = next;
	}
	double m = mean(total, n);
	ey_drop(total);
	return ey_some(opt, ey_f2v(m));
}
