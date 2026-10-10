// Command clidoc writes the docs site's reference for the eye command,
// docs/reference/cli.md, from what `eye help` and each `eye <command> -h`
// print, so the page always says what the command does. Run it from the
// repository root with `task generate`.
package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const out = "docs/reference/cli.md"

var commandLine = regexp.MustCompile(`^  (\w+) +(.+)$`)

func main() {
	dir, err := os.MkdirTemp("", "clidoc")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)
	eye := filepath.Join(dir, "eye")
	if b, err := exec.Command("go", "build", "-o", eye, "./cmd/eye").CombinedOutput(); err != nil {
		log.Fatalf("building eye: %v\n%s", err, b)
	}
	help, _ := run(eye, "help")
	page, err := Page(help, func(cmd string) string { usage, _ := run(eye, cmd, "-h"); return usage })
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(out, []byte(page), 0o644); err != nil {
		log.Fatal(err)
	}
}

// Page returns the reference page, given what `eye help` prints and a way to
// get each command's usage.
func Page(help string, usage func(cmd string) string) (string, error) {
	var b strings.Builder
	b.WriteString("---\ntitle: The eye command\n---\n\n")
	b.WriteString("<!-- Generated from `eye help` and `eye <command> -h` by `task generate`. Don't edit. -->\n\n")
	b.WriteString("# The eye command\n\n")
	b.WriteString("`eye` is the one tool for Eyelang: it checks, formats, tests and runs programs, writes their API ")
	b.WriteString("reference, and is the language server editors talk to. A program is a directory of `.eye` files; ")
	b.WriteString("an `eye.toml` file marks its root.\n\n")
	b.WriteString("| Command | What it does |\n|---|---|\n")
	var cmds []string
	for _, line := range strings.Split(help, "\n") {
		m := commandLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		fmt.Fprintf(&b, "| [`eye %s`](#%s) | %s |\n", m[1], m[1], m[2])
		cmds = append(cmds, m[1])
	}
	if len(cmds) == 0 {
		return "", fmt.Errorf("eye help listed no commands:\n%s", help)
	}
	b.WriteString("\n")
	for _, c := range cmds {
		fmt.Fprintf(&b, "## eye %s {#%s}\n\n", c, c)
		switch c {
		case "help", "version":
			fmt.Fprintf(&b, "```text\nusage: eye %s\n```\n\n", c)
			continue
		}
		fmt.Fprintf(&b, "```text\n%s\n```\n\n", strings.TrimSpace(usage(c)))
	}
	return strings.TrimRight(b.String(), "\n") + "\n", nil
}

// run returns what a command printed, on both streams. Usage messages exit
// with status 2, so the status isn't an error here.
func run(name string, args ...string) (string, error) {
	var buf bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Stdout, cmd.Stderr = &buf, &buf
	err := cmd.Run()
	return buf.String(), err
}
