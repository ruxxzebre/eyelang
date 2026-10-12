// Strings: build one long string with a strings.Builder, split it apart
// again, and look at the pieces.
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"
)

func main() {
	n := 1_000_000
	if len(os.Args) > 1 {
		if v, err := strconv.Atoi(os.Args[1]); err == nil {
			n = v
		}
	}
	var b strings.Builder
	for i := range n {
		b.WriteString("item")
		b.WriteString(strconv.Itoa(i % 1000))
		b.WriteByte(':')
		b.WriteString(strconv.Itoa(i))
		b.WriteByte(';')
	}
	text := b.String()
	parts := strings.Split(text, ";")
	chars, sevens := 0, 0
	for _, part := range parts {
		chars += utf8.RuneCountInString(part) // Eyelang's size counts characters
		if strings.HasPrefix(part, "item7") {
			sevens++
		}
	}
	fmt.Printf("%d characters, %d parts, %d in parts, %d start with item7\n",
		utf8.RuneCountInString(text), len(parts), chars, sevens)
}
