package check

import (
	"sort"
	"strings"
)

// StdModules returns the import paths of the standard library's modules,
// such as "ai" and "http", in order.
func StdModules() []string {
	entries, _ := std.ReadDir("std") // embedded, so it can't fail
	var out []string
	for _, e := range entries {
		if name, ok := strings.CutSuffix(e.Name(), ".eye"); ok {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}
