package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"eyelang/internal/codegen"
)

// The exit test of 6.4 (ROADMAP.md, #56): eye run serves the lab's Notify, a
// Miniflux written in Eyelang and Echo, a contract that sends every type of
// the lab's contracts back; a Go program built against the clients that
// eye gen go writes sends a value of each type and checks it comes back
// unchanged, through eye run and through the generated server stub; and a
// TypeScript program does the same with eye gen ts, when node and tsc are
// on PATH. The programs are in testdata/roundtrip/driver.
func TestGenRoundTrip(t *testing.T) {
	secrets := map[string]string{"NOTIFY_TOKEN": "notify-secret", "ECHO_TOKEN": "echo-secret", "MINIFLUX_KEY": "miniflux-secret"}
	for name, v := range secrets {
		t.Setenv(name, v)
	}
	modules := []string{"notify", "echo", "miniflux"}
	for _, m := range modules {
		t.Setenv(strings.ToUpper(m)+"_PORT", "0") // any free port; eye run names it
		t.Setenv(strings.ToUpper(m)+"_URL", "")   // the program answers its own calls
	}

	lab := roundTripLab(t)
	srv, out := eyeLockedProcess(t, "run", lab)
	defer stopProcess(t, srv, out, 0)
	serving := regexp.MustCompile(`eye run: serving (\w+)\.\w+ on :(\d+)`)
	urls := map[string]string{}
	waitUntil(t, "eye run serves Notify, Echo and Miniflux", out, func() bool {
		for _, m := range serving.FindAllStringSubmatch(out.String(), -1) {
			urls[strings.ToUpper(m[1])+"_URL"] = "http://127.0.0.1:" + m[2]
		}
		return len(urls) == len(modules)
	})
	env := os.Environ()
	for name, v := range urls {
		env = append(env, name+"="+v)
	}

	t.Run("go", func(t *testing.T) {
		goCmd, err := exec.LookPath("go")
		if err != nil {
			t.Skip("go isn't on PATH, so the Go client isn't built")
		}
		dir := t.TempDir()
		gen(t, "go", lab, dir)
		writeFile(t, filepath.Join(dir, "go.mod"), "module roundtrip\n\ngo 1.22\n")
		copyFile(t, "testdata/roundtrip/driver/main.go", filepath.Join(dir, "driver", "main.go"))
		goEnv := append(env, "GOTOOLCHAIN=local", "GOFLAGS=-mod=mod")
		for _, args := range [][]string{{"vet", "./..."}, {"run", "./driver"}} {
			cmd := exec.Command(goCmd, args...)
			cmd.Dir, cmd.Env = dir, goEnv
			got, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("go %s: %v\n%s\neye run said:\n%s", strings.Join(args, " "), err, got, out.String())
			}
			if args[0] == "run" {
				t.Logf("%s", bytes.TrimSpace(got))
			}
		}
	})

	t.Run("ts", func(t *testing.T) {
		tsc, err := exec.LookPath("tsc")
		if err != nil {
			t.Skip("tsc isn't on PATH, so the TypeScript client isn't tried; npm install -g typescript adds it")
		}
		node, err := exec.LookPath("node")
		if err != nil {
			t.Skip("node isn't on PATH, so the TypeScript client isn't tried")
		}
		dir := t.TempDir()
		gen(t, "ts", lab, dir)
		copyFile(t, "testdata/roundtrip/driver/main.ts", filepath.Join(dir, "main.ts"))
		outDir := filepath.Join(dir, "out")
		args := append(codegen.TSCArgs(outDir), filepath.Join(dir, "main.ts"))
		if got, err := exec.Command(tsc, args...).CombinedOutput(); err != nil {
			t.Fatalf("tsc: %v\n%s", err, got)
		}
		writeFile(t, filepath.Join(outDir, "package.json"), `{"type": "module"}`+"\n")
		cmd := exec.Command(node, filepath.Join(outDir, "main.js"))
		cmd.Env = env
		got, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("node: %v\n%s\neye run said:\n%s", err, got, out.String())
		}
		t.Logf("%s", bytes.TrimSpace(got))
	})
}

// roundTripLab is a copy of the lab with Echo, the program that serves it,
// and a Miniflux in Eyelang, whose contract gets a port so that it can be
// served.
func roundTripLab(t *testing.T) string {
	t.Helper()
	files := map[string]string{"lab.eye": "import notifier\nimport echoserver\nimport fakeminiflux\n"}
	for _, path := range []string{"echo/echo.eye", "echoserver/echoserver.eye", "fakeminiflux/fakeminiflux.eye"} {
		data, err := os.ReadFile(filepath.Join("testdata/roundtrip", path))
		if err != nil {
			t.Fatal(err)
		}
		files[path] = string(data)
	}
	contract, err := os.ReadFile("../../testdata/lab/miniflux/miniflux.eye")
	if err != nil {
		t.Fatal(err)
	}
	const attrs = "@http(names: snake)\n@auth(header: \"X-Auth-Token\""
	if !bytes.Contains(contract, []byte(attrs)) {
		t.Fatalf("the lab's Miniflux contract no longer starts with %q", attrs)
	}
	files["miniflux/miniflux.eye"] = strings.Replace(string(contract), attrs, "@port(3002)\n"+attrs, 1)
	return labCopy(t, "roundtrip", files)
}

// gen runs eye gen go|ts on the program, writing a file for each module with
// contracts into dir.
func gen(t *testing.T, target, program, dir string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if code := run([]string{"gen", target, "--out", dir, program}, &stdout, &stderr); code != 0 {
		t.Fatalf("eye gen %s exited with %d:\n%s", target, code, stderr.String())
	}
}

func writeFile(t *testing.T, path, src string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
}

func copyFile(t *testing.T, from, to string) {
	t.Helper()
	data, err := os.ReadFile(from)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, to, string(data))
}
