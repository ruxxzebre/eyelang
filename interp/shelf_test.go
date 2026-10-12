package interp

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

func setOf(vals ...int64) *Set {
	s := &Set{}
	for _, v := range vals {
		s = s.added(v)
	}
	return s
}

// TestAddedKeepsSets checks that adding to a set in place never changes a
// set another value holds, and that an older set doesn't see what a newer
// one on its shelf added.
func TestAddedKeepsSets(t *testing.T) {
	a := setOf(1)
	b := a.added(int64(2)) // in place: a fills every slot
	c := a.added(int64(3)) // copies: b holds a's next slot
	d := a.added(int64(2)) // 2 is on a's shelf, but past a's length
	e := b.added(int64(1)) // already there
	if b.shelf != a.shelf || c.shelf == a.shelf {
		t.Errorf("b should share a's shelf and c should not")
	}
	if a.has(int64(2)) || a.has(int64(3)) {
		t.Errorf("a = %s sees elements added after it", show(a))
	}
	for _, check := range []struct {
		s    *Set
		want string
	}{{a, "{1}"}, {b, "{1, 2}"}, {c, "{1, 3}"}, {d, "{1, 2}"}, {e, "{1, 2}"}} {
		if got := show(check.s); got != check.want {
			t.Errorf("got %s, want %s", got, check.want)
		}
	}
	if !d.has(int64(2)) || d.has(int64(3)) || !equal(b, d) || equal(a, b) {
		t.Errorf("d = %s holds the wrong elements", show(d))
	}
	if keyOf(a) != keyOf(newSet([]Value{int64(1)})) {
		t.Errorf("a's key %q reads elements past its length", keyOf(a))
	}
}

// TestWithKeepsMaps does the same for maps, including replacing a value.
func TestWithKeepsMaps(t *testing.T) {
	a := newMap([]Value{"one"}, []Value{int64(1)})
	b := a.with("two", int64(2))   // in place
	c := a.with("two", int64(22))  // "two" is past a's length: copies
	d := b.with("one", int64(11))  // replaces: shares b's shelf
	e := b.with("three", int64(3)) // in place after b
	if _, ok := a.get("two"); ok {
		t.Errorf("a sees a key added after it")
	}
	if d.shelf != b.shelf {
		t.Errorf("replacing a value copied the keys")
	}
	for _, check := range []struct {
		m    *Map
		want string
	}{
		{a, `{ "one": 1 }`},
		{b, `{ "one": 1, "two": 2 }`},
		{c, `{ "one": 1, "two": 22 }`},
		{d, `{ "one": 11, "two": 2 }`},
		{e, `{ "one": 1, "two": 2, "three": 3 }`},
	} {
		if got := show(check.m); got != check.want {
			t.Errorf("got %s, want %s", got, check.want)
		}
	}
	if v, _ := c.get("two"); v != int64(22) {
		t.Errorf("c's two is %v", v)
	}
}

// TestReplacingKeepsMaps replaces values in maps that share their keys and
// values, then changes the older maps too, in both orders: no map may see a
// change made to another, and each keeps its keys in insertion order.
func TestReplacingKeepsMaps(t *testing.T) {
	for _, n := range []int{3, 100, 5_000} { // values in the tail, and in the tree
		var keys, vals []Value
		for i := range n {
			keys, vals = append(keys, fmt.Sprint("k", i)), append(vals, int64(i))
		}
		a := newMap(keys, vals)
		want := func(m *Map, changed map[int]Value, extra ...string) {
			t.Helper()
			var b strings.Builder
			b.WriteString("{ ")
			for i := range n {
				if i > 0 {
					b.WriteString(", ")
				}
				v, ok := changed[i]
				if !ok {
					v = int64(i)
				}
				fmt.Fprintf(&b, "%q: ", fmt.Sprint("k", i))
				writeShow(&b, v, true)
			}
			for _, e := range extra {
				b.WriteString(", " + e)
			}
			b.WriteString(" }")
			if got := show(m); got != b.String() {
				t.Errorf("n = %d: got %.200s, want %.200s", n, got, b.String())
			}
		}
		last := n - 1
		b := a.with("k0", int64(-1))            // replace in the oldest map
		c := a.with(fmt.Sprint("k", last), "x") // replace again from the same map
		d := b.with("k1", "y")                  // replace in the newer map
		e := a.with("new", int64(7))            // the oldest map grows: fills the shared shelf
		f := b.with("new", int64(8))            // so b's growth copies
		g := e.with("k0", "z")                  // and replacing in the grown map leaves e alone
		want(a, nil)
		want(b, map[int]Value{0: int64(-1)})
		want(c, map[int]Value{last: "x"})
		want(d, map[int]Value{0: int64(-1), 1: "y"})
		want(e, nil, `"new": 7`)
		want(f, map[int]Value{0: int64(-1)}, `"new": 8`)
		want(g, map[int]Value{0: "z"}, `"new": 7`)
		if equal(a, b) || !equal(a, newMap(keys, vals)) || keyOf(b) == keyOf(a) {
			t.Errorf("n = %d: equality sees a replaced value wrongly", n)
		}
		if v, ok := a.get("new"); ok {
			t.Errorf("n = %d: a sees a key added after it: %v", n, v)
		}
	}
}

// TestAddedFromTasks adds to one set and one map from many goroutines at
// once. Run it with -race.
func TestAddedFromTasks(t *testing.T) {
	base := setOf(0)
	mbase := newMap([]Value{int64(0)}, []Value{"zero"})
	var wg sync.WaitGroup
	sets := make([]*Set, 64)
	maps := make([]*Map, 64)
	for i := range sets {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sets[i] = base.added(int64(i + 1))
			maps[i] = mbase.with(int64(i+1), "x")
			_ = base.has(int64(i))
		}()
	}
	wg.Wait()
	for i := range sets {
		if want := fmt.Sprintf("{0, %d}", i+1); show(sets[i]) != want {
			t.Errorf("task %d got %s, want %s", i, show(sets[i]), want)
		}
		if want := fmt.Sprintf(`{ 0: "zero", %d: "x" }`, i+1); show(maps[i]) != want {
			t.Errorf("task %d got %s, want %s", i, show(maps[i]), want)
		}
	}
	if show(base) != "{0}" || show(mbase) != `{ 0: "zero" }` {
		t.Errorf("base changed to %s, %s", show(base), show(mbase))
	}
}

// TestSetAndMapGrowInPlace builds a set and a map of 100,000 elements one
// at a time and counts the shelves they went through. Copying on every
// insertion, as before, made a new shelf each time and the build quadratic;
// doubling the room each time it runs out takes a few dozen.
func TestSetAndMapGrowInPlace(t *testing.T) {
	const n = 100_000
	s, m := &Set{}, &Map{}
	setShelves, mapShelves := 0, 0
	for i := range int64(n) {
		before, mbefore := s.shelf, m.shelf
		s = s.added(i)
		m = m.with(i, i*i)
		if s.shelf != before {
			setShelves++
		}
		if m.shelf != mbefore {
			mapShelves++
		}
	}
	if len(s.Elems) != n || !s.has(int64(n-1)) || len(m.Keys) != n {
		t.Fatalf("built %d elements and %d entries", len(s.Elems), len(m.Keys))
	}
	if v, _ := m.get(int64(300)); v != int64(90_000) {
		t.Errorf("m[300] = %v", v)
	}
	if setShelves > 20 || mapShelves > 20 {
		t.Errorf("building took %d set shelves and %d map shelves, want at most 20", setShelves, mapShelves)
	}
}
