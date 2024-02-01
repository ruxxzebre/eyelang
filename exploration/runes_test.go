package tests

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"testing"
)

func TestRuneReader(t *testing.T) {
	input := "some_string 4242 😐"
	reader := strings.NewReader(input)
	runeReader := bufio.NewReader(reader)

	for {
		rune, size, err := runeReader.ReadRune()
		fmt.Printf("rune: %s size: %d err: %v\n", string(rune), size, err)
		if err == io.EOF {
			return
		}
	}
}
