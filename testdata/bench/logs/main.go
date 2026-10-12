// Text: generate a server log, then parse it line by line and sum it up per
// service: requests, mean and worst latency, and errors.
package main

import (
	"fmt"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"
)

type stats struct{ requests, total, worst, errors int }

func generate(n int) string {
	services := []string{"auth", "billing", "search", "cart", "users", "mail", "images", "orders"}
	levels := []string{"INFO", "INFO", "INFO", "WARN", "ERROR"}
	var b strings.Builder
	seed := 1
	for i := range n {
		seed = (seed*1103515245 + 12345) % 2147483648
		service := services[seed/8%8]
		fmt.Fprintf(&b, "%d %s service=%s ms=%d path=/api/%s/%d\n",
			i, levels[seed/64%5], service, seed/512%900+1, service, seed%10_000)
	}
	return b.String()
}

// value is the value of a key=value field.
func value(field string) string {
	if parts := strings.Split(field, "="); len(parts) == 2 {
		return parts[1]
	}
	return ""
}

func main() {
	n := 300_000
	if len(os.Args) > 1 {
		if v, err := strconv.Atoi(os.Args[1]); err == nil {
			n = v
		}
	}
	text := generate(n)
	all := map[string]stats{}
	bad := 0
	for line := range strings.Lines(text) {
		fields := strings.Split(strings.TrimSuffix(line, "\n"), " ")
		if len(fields) < 4 {
			bad++
			continue
		}
		latency, err := strconv.Atoi(value(fields[3]))
		if err != nil {
			bad++
			continue
		}
		name := value(fields[2])
		s := all[name]
		s.requests++
		s.total += latency
		s.worst = max(s.worst, latency)
		if fields[1] == "ERROR" {
			s.errors++
		}
		all[name] = s
	}
	for _, name := range slices.Sorted(maps.Keys(all)) {
		s := all[name]
		fmt.Printf("%s: %d requests, mean %d ms, worst %d ms, %d errors\n",
			name, s.requests, s.total/s.requests, s.worst, s.errors)
	}
	fmt.Printf("%d lines not understood\n", bad)
}
