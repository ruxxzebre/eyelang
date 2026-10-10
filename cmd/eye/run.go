package main

import (
	"fmt"
	"io"
	"os"

	"eyelang/check"
	"eyelang/interp"
)

// runRun implements `eye run [program-directory] [-- arguments…]`.
func runRun(args []string, stdout, stderr io.Writer) int {
	dir := "."
	var progArgs []string
	for i, a := range args {
		if a == "--" {
			progArgs = args[i+1:]
			args = args[:i]
			break
		}
	}
	switch len(args) {
	case 0:
	case 1:
		dir = args[0]
		if info, err := os.Stat(dir); err == nil && !info.IsDir() {
			fmt.Fprintln(stderr, "eye run takes the program's directory; its .eye files are one module")
			return exitUsage
		}
	default:
		fmt.Fprintln(stderr, "usage: eye run [program-directory] [-- arguments…]")
		return exitUsage
	}
	prog, diags := check.Program(dir)
	if len(diags) > 0 {
		for _, d := range diags {
			fmt.Fprintln(stderr, d)
		}
		return exitErrors
	}
	return interp.RunMain(prog, append([]string{dir}, progArgs...), stdout, stderr)
}
