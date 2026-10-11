package interp

import (
	"io"
	"testing"

	"eyelang/check"
	"eyelang/ir"
)

// TestSavedRecordsKeepTheirType saves records of two types with the same
// fields and reads them back: each keeps its type, and so its Show. A record
// saved without a type comes back without one: nothing guesses it from its
// fields.
func TestSavedRecordsKeepTheirType(t *testing.T) {
	prog, diags := check.Program("../testdata/programs/traits")
	for _, d := range diags {
		t.Fatal(d)
	}
	in := New(prog, io.Discard, io.Discard)
	pence := prog.Root.Types["Pence"]
	money := prog.Root.Types["Money"]
	for _, typ := range []*ir.Type{pence, money} {
		r := &Record{Names: []string{"cents"}, Vals: []Value{int64(40)}, Type: typ}
		back := in.decode(in.encode(r, ir.Pos{}), ir.Pos{})
		got, ok := back.(*Record)
		if !ok || got.Type != typ {
			t.Errorf("a saved %s came back as %#v", typ.Name, back)
		}
	}
	if got := in.display(in.decode(in.encode(&Record{Names: []string{"cents"}, Vals: []Value{int64(40)}, Type: pence}, ir.Pos{}), ir.Pos{})); got != "40p" {
		t.Errorf("a saved Pence shows as %q, want 40p", got)
	}
	untyped := in.decode(in.encode(&Record{Names: []string{"cents"}, Vals: []Value{int64(40)}}, ir.Pos{}), ir.Pos{})
	if in.typeOf(untyped) != nil {
		t.Errorf("a record saved without a type came back as a %s", in.typeOf(untyped).Name)
	}
}
