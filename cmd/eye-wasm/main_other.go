//go:build !(js && wasm)

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "eye-wasm runs in a browser: build it with GOOS=js GOARCH=wasm go build ./cmd/eye-wasm, or task playground")
	os.Exit(2)
}
