// Command specexamples writes every Eyelang example in SPECS.md to
// testdata/syntax/valid, where the conformance suite parses each one with both
// parsers. Run it from the repository root with `task generate`.
package main

import (
	"log"
	"os"
	"path/filepath"
	"strconv"

	"eyelang/internal/specdoc"
)

const dir = "testdata/syntax/valid"

func main() {
	spec, err := os.ReadFile("SPECS.md")
	if err != nil {
		log.Fatal(err)
	}
	old, _ := filepath.Glob(filepath.Join(dir, "spec-*.eye"))
	for _, f := range old {
		if err := os.Remove(f); err != nil {
			log.Fatal(err)
		}
	}
	for _, ex := range specdoc.Examples(spec) {
		header := "# SPECS.md section " + strconv.Itoa(ex.Section) + ", example " + strconv.Itoa(ex.Index) + ".\n"
		if ex.Wrapped {
			header += "# The example is a list of statements, wrapped in a function.\n"
		}
		if err := os.WriteFile(filepath.Join(dir, ex.Name), []byte(header+ex.Source), 0o644); err != nil {
			log.Fatal(err)
		}
	}
}

