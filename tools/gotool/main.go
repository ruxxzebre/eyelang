// Command gotool runs the Go toolchain this repository pins
// (internal/gotool), downloading it once into the user's cache and checking
// its checksum. `eye build --runner` builds with the same Go:
//
//	go run ./tools/gotool version
//	go run ./tools/gotool --root     # its GOROOT, fetching it if needed
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"eyelang/internal/gotool"
)

func main() {
	root, err := gotool.Root(os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gotool:", err)
		os.Exit(1)
	}
	args := os.Args[1:]
	if len(args) == 1 && args[0] == "--root" {
		fmt.Println(root)
		return
	}
	exe, _ := gotool.Path(nil)
	cmd := exec.Command(exe, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = append(os.Environ(), "GOROOT="+root, "GOTOOLCHAIN=local")
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			os.Exit(exit.ExitCode())
		}
		fmt.Fprintln(os.Stderr, "gotool:", err)
		os.Exit(1)
	}
}
