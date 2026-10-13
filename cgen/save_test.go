package cgen

import (
	"bytes"
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"eyelang/interp"
	"eyelang/lir"
)

// saveCheck builds the binary that checks saving for the program in dir
// (Options.SaveCheck) and runs it in mode with input on stdin, with the
// runtime's counts of objects: it gives each line it wrote, and fails if an
// object wasn't freed.
func saveCheck(t *testing.T, exe, mode, input string) []string {
	t.Helper()
	cmd := exec.Command(exe)
	cmd.Env = append(os.Environ(), "EYE_SAVE_CHECK="+mode, "EYE_ALLOCS=1")
	cmd.Stdin = strings.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("%s %s: %v\n%s%s", exe, mode, err, stdout.String(), stderr.String())
	}
	m := allocLine.FindStringSubmatch(stderr.String())
	if m == nil || m[1] != m[2] {
		t.Errorf("%s: objects not freed: %s", mode, stderr.String())
	}
	return strings.Split(strings.TrimSpace(stdout.String()), "\n")
}

// TestSavedValuesRoundTrip saves every module-level let of
// cgen/testdata/saved, a value of each kind of saved type, natively and in
// the interpreter: compiled code writes the interpreter's bytes exactly,
// the interpreter reads them back as the same value, and compiled code
// reads the interpreter's bytes back as the same value and writes the same
// bytes again. This is how the runner and compiled code will hand each
// other values (ROADMAP 7.6).
func TestSavedValuesRoundTrip(t *testing.T) {
	needZig(t)
	if HostTarget() == "" {
		t.Skip("eye build doesn't target this machine")
	}
	prog := load(t, "testdata/saved")
	low, errs := lir.Lower(prog)
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	files, err := Generate(prog, low, Options{SaveCheck: true})
	if err != nil {
		t.Fatal(err)
	}
	in := interp.New(prog, io.Discard, io.Discard)
	want := map[string][]byte{}
	values := map[string]interp.Value{}
	var input strings.Builder
	for _, g := range prog.Root.Globals {
		v, err := in.Global(g.Name)
		if err != nil {
			t.Fatal(err)
		}
		data, err := in.Encode(v)
		if err != nil {
			t.Fatalf("the interpreter can't save %s: %v", g.Name, err)
		}
		want[g.Name], values[g.Name] = data, v
		input.WriteString(g.Name + " " + hex.EncodeToString(data) + "\n")
	}
	for _, debug := range []bool{true, false} {
		exe := filepath.Join(t.TempDir(), "saved")
		if err := Compile(append(Runtime(), files...), BuildOptions{Out: exe, Debug: debug}); err != nil {
			t.Fatal(err)
		}
		seen := 0
		for _, line := range saveCheck(t, exe, "save", "") {
			name, h, _ := strings.Cut(line, " ")
			got, err := hex.DecodeString(h)
			if err != nil || want[name] == nil {
				t.Fatalf("the binary wrote %q", line)
			}
			seen++
			if !bytes.Equal(got, want[name]) {
				t.Errorf("%s: compiled code saves\n\t%x\nthe interpreter\n\t%x", name, got, want[name])
				continue
			}
			back, err := in.Decode(got)
			if err != nil || !interp.Equal(back, values[name]) {
				t.Errorf("%s: the interpreter reads compiled code's bytes as %s (%v), not %s", name, interp.Show(back), err, interp.Show(values[name]))
			}
		}
		if seen != len(want) {
			t.Errorf("the binary saved %d values, not %d", seen, len(want))
		}
		seen = 0
		for _, line := range saveCheck(t, exe, "load", input.String()) {
			parts := strings.SplitN(line, " ", 4)
			if len(parts) < 3 || want[parts[0]] == nil {
				t.Fatalf("the binary wrote %q", line)
			}
			seen++
			name := parts[0]
			if parts[1] == "error" {
				t.Errorf("%s: compiled code can't read the interpreter's bytes: %s", name, strings.Join(parts[2:], " "))
				continue
			}
			again, _ := hex.DecodeString(parts[1])
			shown, _ := hex.DecodeString(parts[3])
			if parts[2] != "1" {
				t.Errorf("%s: compiled code reads the interpreter's bytes as %s, not %s", name, shown, interp.Show(values[name]))
			}
			if !bytes.Equal(again, want[name]) {
				t.Errorf("%s: what compiled code read it saves again as\n\t%x\nnot\n\t%x", name, again, want[name])
			}
		}
		if seen != len(want) {
			t.Errorf("the binary read %d values, not %d", seen, len(want))
		}
	}
}
