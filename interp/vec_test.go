package interp

import (
	"math/rand/v2"
	"slices"
	"testing"
)

// TestVecKeepsValues sets and appends at random, keeping every version, and
// checks each version against a plain slice made for it: no later set or
// append, to it or to a version made from it, may change what it reads.
func TestVecKeepsValues(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	type version struct {
		v    vec
		want []Value
	}
	versions := []version{{}}
	for range 20_000 {
		// Mostly grow the newest version, but often go back to an old one,
		// so that versions branch both in the tail and in the tree.
		from := versions[len(versions)-1]
		if r.IntN(4) == 0 {
			from = versions[r.IntN(len(versions))]
		}
		next := version{want: slices.Clone(from.want)}
		x := Value(int64(r.IntN(1_000_000)))
		if n := from.v.n; n > 0 && r.IntN(2) == 0 {
			i := r.IntN(n)
			next.v = from.v.set(i, x)
			next.want[i] = x
		} else {
			next.v = from.v.push(x)
			next.want = append(next.want, x)
		}
		versions = append(versions, next)
	}
	for i, ver := range versions {
		if ver.v.n != len(ver.want) {
			t.Fatalf("version %d has %d values, want %d", i, ver.v.n, len(ver.want))
		}
		if got := ver.v.values(); !slices.Equal(got, ver.want) {
			t.Fatalf("version %d reads %v, want %v", i, got, ver.want)
		}
		for j, w := range ver.want {
			if got := ver.v.get(j); got != w {
				t.Fatalf("version %d at %d reads %v, want %v", i, j, got, w)
			}
		}
	}
}

// TestVecGrowsDeep appends past the sizes where the tree gains a level and
// sets a value in each level's range.
func TestVecGrowsDeep(t *testing.T) {
	const n = 40_000 // past 32 + 32*32 + 32*32*32
	var v vec
	for i := range n {
		v = v.push(int64(i))
	}
	w := v
	for _, i := range []int{0, 31, 32, 1055, 1056, 33_791, 33_792, n - 1} {
		w = w.set(i, int64(-i))
	}
	for i := range n {
		want := int64(i)
		if v.get(i) != want {
			t.Fatalf("v[%d] = %v after w was set", i, v.get(i))
		}
		if i == 0 || i == 31 || i == 32 || i == 1055 || i == 1056 || i == 33_791 || i == 33_792 || i == n-1 {
			want = -want
		}
		if w.get(i) != want {
			t.Fatalf("w[%d] = %v, want %v", i, w.get(i), want)
		}
	}
}
