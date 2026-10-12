package wire

import (
	"slices"
	"testing"

	"eyelang/ir"
)

func TestNames(t *testing.T) {
	for _, c := range []struct {
		name  string
		snake bool
		want  string
	}{
		{"dedupeKey", true, "dedupe_key"},
		{"Normal", true, "normal"},
		{"NotAllowed", true, "not_allowed"},
		{"dedupeKey", false, "dedupeKey"},
	} {
		if got := Name(c.name, c.snake); got != c.want {
			t.Errorf("Name(%q, %v) = %q, want %q", c.name, c.snake, got, c.want)
		}
	}
	two := &ir.Variant{Name: "Pair", Fields: []ir.VariantField{{}, {}}}
	one := &ir.Variant{Name: "Text", Fields: []ir.VariantField{{}}}
	named := &ir.Variant{Name: "Accepted", Fields: []ir.VariantField{{Name: "id"}}}
	if got := []string{FieldName(two, 0), FieldName(two, 1), FieldName(one, 0), FieldName(named, 0)}; !slices.Equal(got, []string{"value1", "value2", "value", "id"}) {
		t.Errorf("field names %v", got)
	}
}

func TestRequests(t *testing.T) {
	if got := PathParams("/v1/categories/:category/entries/:id"); !slices.Equal(got, []string{"category", "id"}) {
		t.Errorf("PathParams = %v", got)
	}
	if !InQuery("GET") || !InQuery("DELETE") || InQuery("POST") {
		t.Error("GET and DELETE send their arguments in the query, and only they")
	}
	op := &ir.Op{HTTP: &ir.OpHTTP{Status: map[int]string{400: "Refused", 403: "Refused", 202: "Pending"}}}
	for variant, want := range map[string]int{"Refused": 400, "Pending": 202, "Accepted": 200} {
		if got := ReplyStatus(op, variant); got != want {
			t.Errorf("ReplyStatus(%s) = %d, want %d", variant, got, want)
		}
	}
	if got := Statuses(op); !slices.Equal(got, []int{202, 400, 403}) {
		t.Errorf("Statuses = %v", got)
	}
	if KeyHeader(op) != "Idempotency-Key" || KeyHeader(&ir.Op{KeyHeader: "X-Key"}) != "X-Key" {
		t.Error("a key goes in Idempotency-Key unless @key names a header")
	}
}
