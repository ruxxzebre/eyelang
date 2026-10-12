// Command zig runs the zig this repository pins (internal/zig), downloading
// it once into the user's cache and checking its checksum. Every build of C
// in the repository goes through it or through `eye build`, so everyone
// compiles with the same zig:
//
//	go run ./tools/zig cc -O2 hello.c -o hello
//	go run ./tools/zig --path        # where the pinned zig lives, fetching it if needed
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"eyelang/internal/zig"
)

func main() {
	exe, err := zig.Path(os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "zig:", err)
		os.Exit(1)
	}
	args := os.Args[1:]
	if len(args) == 1 && args[0] == "--path" {
		fmt.Println(exe)
		return
	}
	cmd := exec.Command(exe, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			os.Exit(exit.ExitCode())
		}
		fmt.Fprintln(os.Stderr, "zig:", err)
		os.Exit(1)
	}
}
