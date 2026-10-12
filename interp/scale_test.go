package interp

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"eyelang/check"
	"eyelang/ir"
)

// Benchmarks of the idioms whose cost used to grow with the size of the
// data: replacing a map key's value, writing an entity whose type has many
// entities, and building a string with +=. Each should take time in
// proportion to the work it does.
//
//	go test ./interp -run '^$' -bench 'MapCounter|EntityWrite|StringAppend'

// checkSource checks a program made of one file.
func checkSource(tb testing.TB, src string) *ir.Program {
	tb.Helper()
	dir := tb.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.eye"), []byte(src), 0o644); err != nil {
		tb.Fatal(err)
	}
	prog, diags := check.Program(dir)
	for _, d := range diags {
		tb.Fatal(d)
	}
	return prog
}

// runSource runs the tests of prog b.N times, failing on a failed test.
func runSource(b *testing.B, prog *ir.Program) {
	b.Helper()
	for range b.N {
		for _, r := range RunTests(prog, "") {
			if r.Failure != "" {
				b.Fatalf("%s: %s", r.Name, r.Failure)
			}
		}
	}
}

// BenchmarkMapCounter counts 20,000 numbers into 10,000 keys: every second
// insert replaces a key's value. Replacing copied the map's index, which
// made this take 6 to 11 s; now it takes tens of milliseconds.
func BenchmarkMapCounter(b *testing.B) {
	runSource(b, checkSource(b, `
test "map counter" {
  var counts: Map(Int, Int) = {}
  for i in 0..20000 {
    let k = i % 10000
    counts = counts |> insert(k, (counts[k] ?? 0) + 1)
  }
  expect(counts |> size == 10000 && counts[9999] == Some(2))
}
`))
}

// BenchmarkStringAppend builds a string of 100,000 pieces with +=. Go's
// x + y copies both strings, which made this take 18 s; now it takes tens
// of milliseconds.
func BenchmarkStringAppend(b *testing.B) {
	runSource(b, checkSource(b, `
test "string append" {
  var out = ""
  for i in 0..100000 {
    out += "x{i},"
  }
  expect(out |> startsWith("x0,x1,") && out |> endsWith(",x99999,"))
}
`))
}

// BenchmarkEntityWrite writes one entity again and again when its type has
// 1,000 and 10,000 entities: the time per write should be about the same.
// The store keeps an entity type's rows in one map, and a write replaces a
// key's value in it, which used to copy the map.
func BenchmarkEntityWrite(b *testing.B) {
	for _, n := range []int{1_000, 10_000} {
		b.Run(itoa(int64(n))+" entities", func(b *testing.B) {
			in := New(checkSource(b, sleepers), io.Discard, io.Discard)
			counter := in.prog.Root.Entities["Counter"]
			bump := counter.Methods["bump"]
			for i := range n {
				in.rt.callEntity(&EntityRef{Entity: counter, ID: int64(i)}, bump, nil, ir.Pos{})
			}
			first := &EntityRef{Entity: counter, ID: int64(0)}
			b.ResetTimer()
			for range b.N {
				in.rt.callEntity(first, bump, nil, ir.Pos{})
			}
			b.StopTimer()
			state, _, _ := in.rt.load(first, ir.Pos{})
			if v, _ := state.get("n"); v != int64(b.N+1) {
				b.Errorf("Counter(0).n = %v, want %d", v, b.N+1)
			}
		})
	}
}
