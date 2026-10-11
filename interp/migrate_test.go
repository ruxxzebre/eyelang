package interp

import (
	"io"
	"strings"
	"testing"

	"eyelang/check"
	"eyelang/ir"
)

// TestMigrations stores players as versions 1, 2 and 3 of the program in
// testdata/programs/migrations saved them, and loads each as the current
// version declares it.
func TestMigrations(t *testing.T) {
	prog, diags := check.Program("../testdata/programs/migrations")
	for _, d := range diags {
		t.Fatal(d)
	}
	in := New(prog, io.Discard, io.Discard)
	e := prog.Root.Entities["Player"]
	if e.Version != 3 || len(e.Migrations) != 2 {
		t.Fatalf("Player is at version %d with %d migrations", e.Version, len(e.Migrations))
	}
	id := func(name string) Value { return &Variant{V: prog.Root.Variants["PlayerId"], Vals: []Value{name}} }
	ref := func(name string) *EntityRef { return &EntityRef{Entity: e, ID: id(name)} }
	save := func(name string, schema int, fields ...any) {
		r := &Record{Names: []string{"id"}, Vals: []Value{id(name)}}
		for i := 0; i < len(fields); i += 2 {
			r.Names = append(r.Names, fields[i].(string))
			r.Vals = append(r.Vals, fields[i+1])
		}
		in.rt.store.Commit(&Commit{Writes: []Write{{
			Key: entityKey(ref(name)), Entity: entityName(e), State: in.encode(r, ir.Pos{}), Version: 4, Schema: schema,
		}}})
	}
	save("ann", 1, "nickname", " Ann ")
	save("bo", 2, "nick", "Bo")
	save("cy", 3, "name", "Cy", "devices", newSet(nil)) // saved before joined was added
	save("old", 0, "nickname", "Old")                   // saved before rows had versions: version 1
	if err := in.rt.checkVersions(); err != nil {
		t.Fatal(err)
	}

	for name, want := range map[string]string{"ann": "Ann", "bo": "Bo", "cy": "Cy", "old": "Old"} {
		state, version, ok := in.rt.load(ref(name), ir.Pos{})
		if !ok || version != 4 {
			t.Fatalf("%s: loaded %v, version %d", name, ok, version)
		}
		if got, _ := state.get("name"); got != want {
			t.Errorf("%s: name = %v, want %q", name, got, want)
		}
		if got, _ := state.get("joined"); got != int64(0) {
			t.Errorf("%s: joined = %v, want the default 0", name, got)
		}
		if strings.Join(state.Names, ",") != "id,name,devices,joined" {
			t.Errorf("%s: fields %v", name, state.Names)
		}
	}

	// A change saves the row at the current version.
	in.rt.callEntity(ref("ann"), e.Methods["rename"], []argVal{{v: "Annie"}}, ir.Pos{})
	if row, _ := in.rt.store.Load(entityKey(ref("ann"))); row.Schema != 3 {
		t.Errorf("after a change the row is at version %d, want 3", row.Schema)
	}

	// Rows no migration can bring to the current version stop eye run
	// before it starts.
	save("future", 4, "name", "F")
	save("ancient", 0, "nickname", "A")
	e.Migrations = e.Migrations[1:] // as if migrate from 1 were removed
	err := in.rt.checkVersions()
	if err == nil {
		t.Fatal("checkVersions found nothing wrong")
	}
	for _, want := range []string{"1 Player saved at version 4, newer than the program's @version(3)", "Player saved at version 1, and Player has no migrate from 1"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("checkVersions: %v\nwant it to say %q", err, want)
		}
	}
}
