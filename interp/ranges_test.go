package interp

import "testing"

// TestRangesByValue checks that ranges compare, key and show by their bounds
// and whether they include their end, open ends too, which only slices make.
func TestRangesByValue(t *testing.T) {
	r := func(start, end Value, incl bool) *RangeVal { return &RangeVal{Start: start, End: end, Inclusive: incl} }
	same := [][2]*RangeVal{
		{r(int64(1), int64(5), false), r(int64(1), int64(5), false)},
		{r(nil, int64(5), true), r(nil, int64(5), true)},
		{r(int64(2), nil, false), r(int64(2), nil, false)},
	}
	for _, p := range same {
		if !Equal(p[0], p[1]) || keyOf(p[0]) != keyOf(p[1]) {
			t.Errorf("%s and %s should be equal, with keys %q and %q", show(p[0]), show(p[1]), keyOf(p[0]), keyOf(p[1]))
		}
	}
	differ := [][2]*RangeVal{
		{r(int64(1), int64(5), false), r(int64(1), int64(5), true)},
		{r(int64(5), int64(2), false), r(int64(6), int64(1), false)},
		{r(nil, int64(5), false), r(int64(0), int64(5), false)},
		{r(int64(5), nil, false), r(nil, int64(5), false)},
	}
	for _, p := range differ {
		if Equal(p[0], p[1]) || keyOf(p[0]) == keyOf(p[1]) {
			t.Errorf("%s and %s should differ, with keys %q and %q", show(p[0]), show(p[1]), keyOf(p[0]), keyOf(p[1]))
		}
	}
	for want, v := range map[string]*RangeVal{
		"1..5": r(int64(1), int64(5), false), "1..=5": r(int64(1), int64(5), true),
		"..5": r(nil, int64(5), false), "2..": r(int64(2), nil, false), "'a'..='z'": r('a', 'z', true),
	} {
		if got := show(v); got != want {
			t.Errorf("show gives %s, want %s", got, want)
		}
	}
}
