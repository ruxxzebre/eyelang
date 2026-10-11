package main

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var updateIR = flag.Bool("update", false, "rewrite each testdata/ir program's ir.golden and lir.golden from eye check --ir and --lir")

// TestCheckIR compares `eye check --ir` and `eye check --lir` on each program
// in testdata/ir with its ir.golden and lir.golden: the typed IR and the low
// IR of operators, effects, entities and jobs. `go test ./cmd/eye -run
// TestCheckIR -update` rewrites them, and the diff shows what changed.
func TestCheckIR(t *testing.T) {
	dirs, err := filepath.Glob("../../testdata/ir/*")
	if err != nil || len(dirs) == 0 {
		t.Fatalf("no programs in testdata/ir: %v", err)
	}
	for _, dir := range dirs {
		for _, form := range []string{"ir", "lir"} {
			t.Run(filepath.Base(dir)+"/"+form, func(t *testing.T) { checkGolden(t, dir, form) })
		}
	}
}

// checkGolden compares `eye check --FORM DIR` with DIR/FORM.golden.
func checkGolden(t *testing.T, dir, form string) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"check", "--" + form, dir}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d:\n%s%s", code, stdout.String(), stderr.String())
	}
	golden := filepath.Join(dir, form+".golden")
	if *updateIR {
		if err := os.WriteFile(golden, stdout.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("%v (run with -update to write it)", err)
	}
	if !bytes.Equal(stdout.Bytes(), want) {
		t.Errorf("eye check --%s %s differs from %s; run with -update and read the diff:\n%s", form, dir, golden, stdout.String())
	}
}
