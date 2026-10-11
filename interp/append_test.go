package interp

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func ints(l *List) string { return fmt.Sprint(l.Elems) }

// TestAppendedKeepsValues checks that appending in place never changes a
// list another value holds: lists that grow from the same list stay apart.
func TestAppendedKeepsValues(t *testing.T) {
	a := (&List{}).appended(int64(1))
	b := a.appended(int64(2)) // in place: a uses all of the array
	c := a.appended(int64(3)) // copies: b holds a's next slot
	d := b.appended(int64(4)) // in place after b
	e := b.appended(int64(5)) // copies: d holds b's next slot
	for _, check := range []struct {
		l    *List
		want string
	}{{a, "[1]"}, {b, "[1 2]"}, {c, "[1 3]"}, {d, "[1 2 4]"}, {e, "[1 2 5]"}} {
		if got := ints(check.l); got != check.want {
			t.Errorf("got %s, want %s", got, check.want)
		}
	}
	// A list made elsewhere, without spare room, copies too.
	f := &List{Elems: []Value{int64(7)}}
	if g := f.appended(int64(8)); ints(f) != "[7]" || ints(g) != "[7 8]" {
		t.Errorf("f = %s, g = %s", ints(f), ints(g))
	}
}

// TestAppendedFromTasks appends to one list from many goroutines at once:
// each result must be the list with its own element, whichever won the slot.
// Run it with -race.
func TestAppendedFromTasks(t *testing.T) {
	base := (&List{}).appended(int64(0))
	var wg sync.WaitGroup
	results := make([]*List, 64)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = base.appended(int64(i + 1))
		}()
	}
	wg.Wait()
	for i, r := range results {
		if want := fmt.Sprintf("[0 %d]", i+1); ints(r) != want {
			t.Errorf("task %d got %s, want %s", i, ints(r), want)
		}
	}
	if ints(base) != "[0]" {
		t.Errorf("base changed to %s", ints(base))
	}
}

// TestAppendedIsLinear builds a long list one element at a time. Copying
// the list on every append, as before, made this quadratic: minutes, not
// milliseconds.
func TestAppendedIsLinear(t *testing.T) {
	start := time.Now()
	l := &List{}
	for i := range 1_000_000 {
		l = l.appended(int64(i))
	}
	if len(l.Elems) != 1_000_000 || l.Elems[999_999] != int64(999_999) {
		t.Fatalf("built %d elements", len(l.Elems))
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("a million appends took %v", d)
	}
}
