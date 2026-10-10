package syntax

import "testing"

func TestLines(t *testing.T) {
	lines := NewLines([]byte("ab\n\ncd\n"))
	tests := []struct {
		offset int
		want   Position
	}{
		{0, Position{1, 1}},
		{1, Position{1, 2}},
		{2, Position{1, 3}}, // the newline itself
		{3, Position{2, 1}},
		{4, Position{3, 1}},
		{6, Position{3, 3}},
		{7, Position{4, 1}},
		{99, Position{4, 1}},
		{-1, Position{1, 1}},
	}
	for _, tt := range tests {
		if got := lines.Position(tt.offset); got != tt.want {
			t.Errorf("Position(%d) = %v, want %v", tt.offset, got, tt.want)
		}
	}
	if got := lines.Count(); got != 4 {
		t.Errorf("Count() = %d, want 4", got)
	}
	if got := lines.LineStart(3); got != 4 {
		t.Errorf("LineStart(3) = %d, want 4", got)
	}
}
