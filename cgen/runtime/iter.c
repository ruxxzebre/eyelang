/* Iter values and walks, as the interpreter's interp/iter.go has them. An
 * Iter is a source collection and the lazy steps applied to it; a walk is a
 * cursor that pulls one element at a time through the steps, so a pipeline
 * over a very large range computes only what is asked for. A for loop walks
 * anything with the same cursor. */
#include <stdio.h>
#include <stdlib.h>

#include "eye.h"
#include "builtins.h"

/* ---------------------------------------------------------------- Iter values */

static ey_lazy *lazy_alloc(uint32_t n) {
	ey_lazy *it = ey_alloc(sizeof(ey_lazy) + n * sizeof(ey_step), &ey_t_iter);
	it->n = n;
	return it;
}

ey_v ey_lazy_of(ey_v coll, const ey_type *elem) {
	if (EY_OBJ(coll)->ty->kind == EY_ITER) {
		ey_dup(coll);
		return coll;
	}
	ey_lazy *it = lazy_alloc(0);
	ey_dup(coll);
	it->src = coll;
	it->elem = elem;
	return EY_V(it);
}

ey_v ey_lazy_then(ey_v v, int op, ey_v fn, int64_t n, ey_v other, const ey_type *out) {
	ey_lazy *from = (ey_lazy *)(uintptr_t)v;
	ey_lazy *it = lazy_alloc(from->n + 1);
	it->src = from->src;
	it->elem = from->elem;
	ey_dup(it->src);
	for (uint32_t i = 0; i < from->n; i++) {
		it->s[i] = from->s[i];
		ey_dup(it->s[i].fn);
		ey_dup(it->s[i].other);
	}
	ey_step *s = &it->s[from->n];
	s->op = op;
	s->fn = fn;
	s->n = n;
	s->other = other;
	s->out = out;
	ey_dup(fn);
	ey_dup(other);
	return EY_V(it);
}

void ey_lazy_free(ey_obj *o) {
	ey_lazy *it = (ey_lazy *)o;
	ey_drop(it->src);
	for (uint32_t i = 0; i < it->n; i++) {
		ey_drop(it->s[i].fn);
		ey_drop(it->s[i].other);
	}
	ey_free_obj(o, sizeof(ey_lazy) + it->n * sizeof(ey_step));
}

/* ---------------------------------------------------------------- cursors */

typedef struct cursor cursor;

struct cursor {
	/* the source: a collection and where the walk is in it */
	ey_v src;
	const ey_type *elem;
	int64_t pos, end;
	/* the Iter whose steps run, or NULL */
	ey_lazy *lz;
	int64_t *state;  /* per step: what take has left, what drop skips, enumerate's index */
	cursor **sub;    /* per step: zip's other walk, flatMap's walk of the part it is in */
};

static cursor *cur_open(ey_v v, const ey_type *elem);
static bool cur_next(cursor *c, ey_v *out);
static void cur_close(cursor *c);

static void *xcalloc(size_t n, size_t size) {
	void *p = calloc(n ? n : 1, size);
	if (!p) ey_out_of_memory();
	return p;
}

static cursor *cur_open(ey_v v, const ey_type *elem) {
	cursor *c = xcalloc(1, sizeof(cursor));
	ey_obj *o = EY_OBJ(v);
	if (o->ty->kind == EY_ITER) {
		ey_lazy *lz = (ey_lazy *)o;
		ey_dup(v);
		c->lz = lz;
		c->state = xcalloc(lz->n, sizeof(int64_t));
		c->sub = xcalloc(lz->n, sizeof(cursor *));
		for (uint32_t i = 0; i < lz->n; i++) {
			ey_step *s = &lz->s[i];
			if (s->op == EY_STEP_TAKE || s->op == EY_STEP_DROP) c->state[i] = s->n;
			if (s->op == EY_STEP_ZIP) c->sub[i] = cur_open(s->other, s->out->args[1]);
		}
		v = lz->src;
		elem = lz->elem;
		o = EY_OBJ(v);
	}
	ey_dup(v);
	c->src = v;
	c->elem = elem;
	switch (o->ty->kind) {
	case EY_RANGE: {
		ey_agg *r = (ey_agg *)o;
		if (o->ty->args[0]->kind != EY_INT || !EY_IS_SMALL(r->f[0]) || !EY_IS_SMALL(r->f[1]) ||
		    (r->f[2] & (EY_RANGE_NO_START | EY_RANGE_NO_END)))
			ey_fault("<range>", "a range of Ints is needed here");
		c->pos = EY_SMALL_VAL(r->f[0]);
		c->end = EY_SMALL_VAL(r->f[1]) + (r->f[2] & EY_RANGE_INCLUSIVE ? 1 : 0);
		break;
	}
	case EY_STRING:
	case EY_BYTES:
		c->end = ((ey_str *)o)->len;
		break;
	case EY_LIST:
		c->end = ((ey_list *)o)->len;
		break;
	case EY_SET:
	case EY_MAP:
		c->end = ((ey_map *)o)->len;
		break;
	default:
		ey_fault("<iter>", "this value can't be walked");
	}
	return c;
}

/* src_next gives the source's next element, owned. */
static bool src_next(cursor *c, ey_v *out) {
	if (c->pos >= c->end) return false;
	ey_obj *o = EY_OBJ(c->src);
	switch (o->ty->kind) {
	case EY_LIST:
		*out = ((ey_list *)o)->items[c->pos++];
		ey_dup_as(c->elem, *out);
		return true;
	case EY_SET:
		*out = ((ey_map *)o)->keys[c->pos++];
		ey_dup_as(c->elem, *out);
		return true;
	case EY_MAP: {
		ey_map *m = (ey_map *)o;
		ey_agg *t = ey_agg_new(c->elem, 0, 2);
		t->f[0] = m->keys[c->pos];
		t->f[1] = m->vals[c->pos];
		c->pos++;
		ey_dup_as(c->elem->args[0], t->f[0]);
		ey_dup_as(c->elem->args[1], t->f[1]);
		*out = EY_V(t);
		return true;
	}
	case EY_STRING: {
		ey_str *s = (ey_str *)o;
		uint32_t r;
		c->pos += ey_utf8_decode(s->b + c->pos, s->len - c->pos, &r);
		*out = r;
		return true;
	}
	case EY_BYTES:
		*out = (unsigned char)((ey_str *)o)->b[c->pos++];
		return true;
	case EY_RANGE:
		*out = EY_SMALL(c->pos);
		c->pos++;
		return true;
	}
	return false;
}

static ey_v pair(const ey_type *t, ey_v a, ey_v b) {
	ey_agg *p = ey_agg_new(t, 0, 2);
	p->f[0] = a;
	p->f[1] = b;
	return EY_V(p);
}

/* pull gives the next element after step k, owned; k = -1 is the source. */
static bool pull(cursor *c, int k, ey_v *out) {
	if (k < 0) return src_next(c, out);
	ey_step *s = &c->lz->s[k];
	const ey_type *in = k == 0 ? c->lz->elem : c->lz->s[k - 1].out;
	ey_v x;
	switch (s->op) {
	case EY_STEP_MAP:
		if (!pull(c, k - 1, &x)) return false;
		*out = ey_call(s->fn, &x);
		ey_drop_as(in, x);
		return true;
	case EY_STEP_FILTER:
		for (;;) {
			if (!pull(c, k - 1, &x)) return false;
			if (ey_call(s->fn, &x)) {
				*out = x;
				return true;
			}
			ey_drop_as(in, x);
		}
	case EY_STEP_TAKE:
		if (c->state[k] <= 0) return false; /* without asking for one more */
		c->state[k]--;
		return pull(c, k - 1, out);
	case EY_STEP_DROP:
		for (; c->state[k] > 0; c->state[k]--) {
			if (!pull(c, k - 1, &x)) return false;
			ey_drop_as(in, x);
		}
		return pull(c, k - 1, out);
	case EY_STEP_ENUMERATE:
		if (!pull(c, k - 1, &x)) return false;
		*out = pair(s->out, EY_SMALL(c->state[k]), x);
		c->state[k]++;
		return true;
	case EY_STEP_ZIP: {
		ey_v y;
		if (!pull(c, k - 1, &x)) return false;
		if (!cur_next(c->sub[k], &y)) {
			ey_drop_as(in, x);
			return false;
		}
		*out = pair(s->out, x, y);
		return true;
	}
	case EY_STEP_FLATMAP:
		for (;;) {
			if (c->sub[k]) {
				if (cur_next(c->sub[k], out)) return true;
				cur_close(c->sub[k]);
				c->sub[k] = NULL;
			}
			if (!pull(c, k - 1, &x)) return false;
			ey_v part = ey_call(s->fn, &x);
			ey_drop_as(in, x);
			c->sub[k] = cur_open(part, s->out);
			ey_drop(part);
		}
	}
	return false;
}

static bool cur_next(cursor *c, ey_v *out) { return pull(c, c->lz ? (int)c->lz->n - 1 : -1, out); }

static void cur_close(cursor *c) {
	if (c->lz) {
		for (uint32_t i = 0; i < c->lz->n; i++)
			if (c->sub[i]) cur_close(c->sub[i]);
		free(c->state);
		free(c->sub);
		ey_drop(EY_V(c->lz));
	}
	ey_drop(c->src);
	free(c);
}

/* the type of the elements a cursor gives */
static const ey_type *cur_elem(cursor *c) { return c->lz && c->lz->n ? c->lz->s[c->lz->n - 1].out : c->elem; }

ey_v ey_lazy_list(ey_v v, const ey_type *list) {
	cursor *c = cur_open(v, list->args[0]);
	int64_t n = 0, cap = 8;
	ey_v *items = malloc(sizeof(ey_v) * (size_t)cap);
	if (!items) ey_out_of_memory();
	ey_v x;
	while (cur_next(c, &x)) {
		if (n == cap) {
			cap *= 2;
			items = realloc(items, sizeof(ey_v) * (size_t)cap);
			if (!items) ey_out_of_memory();
		}
		items[n++] = x;
	}
	cur_close(c);
	return ey_list_take(list, n, items);
}

/* ---------------------------------------------------------------- the functions that stop early */

ey_v eyb_iter_first(ey_v it, const ey_type *opt) {
	cursor *c = cur_open(it, NULL);
	ey_v x;
	ey_v r = cur_next(c, &x) ? ey_some(opt, x) : ey_none(opt);
	cur_close(c);
	return r;
}

ey_v eyb_iter_find(ey_v it, ey_v f, const ey_type *opt) {
	cursor *c = cur_open(it, NULL);
	const ey_type *et = cur_elem(c);
	ey_v x, r = 0;
	while (!r && cur_next(c, &x)) {
		if (ey_call(f, &x))
			r = ey_some(opt, x);
		else
			ey_drop_as(et, x);
	}
	cur_close(c);
	return r ? r : ey_none(opt);
}

/* how: 0 any, 1 all */
ey_v eyb_iter_test(ey_v it, ey_v f, int how) {
	cursor *c = cur_open(it, NULL);
	const ey_type *et = cur_elem(c);
	ey_v x;
	bool r = how == 1;
	while (cur_next(c, &x)) {
		bool yes = ey_call(f, &x) != 0;
		ey_drop_as(et, x);
		if (how == 0 && yes) {
			r = true;
			break;
		}
		if (how == 1 && !yes) {
			r = false;
			break;
		}
	}
	cur_close(c);
	return r;
}

bool eyb_iter_contains(ey_v it, ey_v y) {
	cursor *c = cur_open(it, NULL);
	const ey_type *et = cur_elem(c);
	ey_v x;
	bool found = false;
	while (!found && cur_next(c, &x)) {
		found = ey_eq(et, x, y);
		ey_drop_as(et, x);
	}
	cur_close(c);
	return found;
}

bool eyb_iter_is_empty(ey_v it) {
	cursor *c = cur_open(it, NULL);
	ey_v x;
	bool more = cur_next(c, &x);
	if (more) ey_drop_as(cur_elem(c), x);
	cur_close(c);
	return !more;
}

/* ---------------------------------------------------------------- for loops */

typedef struct ey_loop {
	ey_obj h;
	cursor *c;
	bool has;
	ey_v next;
} ey_loop;

ey_v ey_loop_new(ey_v coll, const ey_type *elem) {
	ey_loop *l = ey_alloc(sizeof(ey_loop), &ey_t_loop);
	l->c = cur_open(coll, elem);
	l->has = false;
	l->next = 0;
	return EY_V(l);
}

bool ey_loop_more(ey_v v) {
	ey_loop *l = (ey_loop *)(uintptr_t)v;
	if (!l->has) l->has = cur_next(l->c, &l->next);
	return l->has;
}

ey_v ey_loop_next(ey_v v) {
	ey_loop *l = (ey_loop *)(uintptr_t)v;
	if (!l->has && !cur_next(l->c, &l->next)) ey_fault("<loop>", "a for loop went past its end");
	l->has = false;
	return l->next;
}

void ey_loop_free(ey_obj *o) {
	ey_loop *l = (ey_loop *)o;
	if (l->has) ey_drop_as(cur_elem(l->c), l->next);
	cur_close(l->c);
	ey_free_obj(o, sizeof(ey_loop));
}
