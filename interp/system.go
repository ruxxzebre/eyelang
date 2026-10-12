package interp

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"eyelang/ir"
)

// The host functions of the standard modules fs, process and random
// (check/std). In a test, fs and process work in an empty directory of the
// test's own, and random numbers come from a fixed seed, so the test gives
// the same result on every run.

func init() {
	hostFuncs["fs.readText"] = hostReadText
	hostFuncs["fs.readBytes"] = hostReadBytes
	hostFuncs["fs.lines"] = hostLines
	hostFuncs["fs.writeBytes"] = func(in *Interp, p map[string]Value, at ir.Pos) Value {
		return in.fsWrite(p, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, at)
	}
	hostFuncs["fs.writeText"] = func(in *Interp, p map[string]Value, at ir.Pos) Value {
		return in.fsWrite(p, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, at)
	}
	hostFuncs["fs.appendText"] = func(in *Interp, p map[string]Value, at ir.Pos) Value {
		return in.fsWrite(p, os.O_WRONLY|os.O_CREATE|os.O_APPEND, at)
	}
	hostFuncs["fs.exists"] = func(in *Interp, p map[string]Value, at ir.Pos) Value {
		_, err := os.Stat(in.path(p["path"], at))
		return err == nil
	}
	hostFuncs["fs.list"] = hostList
	hostFuncs["fs.makeDir"] = func(in *Interp, p map[string]Value, at ir.Pos) Value {
		path := show(p["path"])
		return in.fsResult(path, os.MkdirAll(in.path(p["path"], at), 0o755), ir.Unit{}, at)
	}
	hostFuncs["fs.remove"] = func(in *Interp, p map[string]Value, at ir.Pos) Value {
		path := show(p["path"])
		return in.fsResult(path, os.Remove(in.path(p["path"], at)), ir.Unit{}, at)
	}
	hostFuncs["process.run"] = hostRun
	hostFuncs["random.int"] = hostRandomInt
	hostFuncs["random.float"] = func(in *Interp, _ map[string]Value, _ ir.Pos) Value {
		return in.rt.random().Float64()
	}
	hostFuncs["random.pick"] = func(in *Interp, p map[string]Value, at ir.Pos) Value {
		xs := in.elems(p["xs"], at)
		if len(xs) == 0 {
			return in.none()
		}
		return in.some(xs[in.rt.random().IntN(len(xs))])
	}
	hostFuncs["random.shuffle"] = func(in *Interp, p map[string]Value, at ir.Pos) Value {
		xs := append([]Value(nil), in.elems(p["xs"], at)...)
		in.rt.random().Shuffle(len(xs), func(i, j int) { xs[i], xs[j] = xs[j], xs[i] })
		return &List{Elems: xs}
	}
}

// workDir is where relative paths and programs run: the process's own
// directory, or in a test, a new empty one.
func (in *Interp) workDir(at ir.Pos) string {
	rt := in.rt
	if !rt.sandbox {
		return ""
	}
	if rt.dir == "" {
		dir, err := os.MkdirTemp("", "eye-test-")
		if err != nil {
			in.faultf(at, "making the test's directory: %v", err)
		}
		rt.dir = dir
	}
	return rt.dir
}

// path resolves a path the program gave.
func (in *Interp) path(v Value, at ir.Pos) string {
	p, ok := v.(string)
	if !ok {
		in.faultf(at, "a path is a String, and this is %s", kindName(v))
	}
	if dir := in.workDir(at); dir != "" && !filepath.IsAbs(p) {
		return filepath.Join(dir, p)
	}
	return p
}

// fsError is the FsError of err, which happened at path as the program
// wrote it.
func (in *Interp) fsError(path string, err error, at ir.Pos) Value {
	t := in.stdModule("fs", at).Types["FsError"]
	variant := func(name string) *ir.Variant {
		for _, v := range t.Variants {
			if v.Name == name {
				return v
			}
		}
		panic("fs.FsError has no variant " + name)
	}
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return &Variant{V: variant("NotFound"), Vals: []Value{path}}
	case errors.Is(err, fs.ErrPermission):
		return &Variant{V: variant("NotAllowed"), Vals: []Value{path}}
	case errors.Is(err, fs.ErrExist):
		return &Variant{V: variant("AlreadyExists"), Vals: []Value{path}}
	}
	reason := err.Error()
	var pe *fs.PathError
	if errors.As(err, &pe) {
		reason = pe.Err.Error() // without the resolved path, which may be a test's directory
	}
	return &Variant{V: variant("SystemError"), Vals: []Value{path, reason}}
}

// fsResult is Ok(ok), or Err with the FsError of err.
func (in *Interp) fsResult(path string, err error, ok Value, at ir.Pos) Value {
	if err != nil {
		return in.errV(in.fsError(path, err, at))
	}
	return in.ok(ok)
}

func hostReadBytes(in *Interp, p map[string]Value, at ir.Pos) Value {
	path := show(p["path"])
	var data []byte
	var err error
	in.blocking(func(context.Context) { data, err = os.ReadFile(in.path(p["path"], at)) })
	if err != nil {
		return in.errV(in.fsError(path, err, at))
	}
	return in.ok(bytesValue(data))
}

func hostReadText(in *Interp, p map[string]Value, at ir.Pos) Value {
	text, failed := in.readText(p, at)
	if failed != nil {
		return failed
	}
	return in.ok(text)
}

// hostLines is fs.lines: the file's text, split as `lines` splits a string.
func hostLines(in *Interp, p map[string]Value, at ir.Pos) Value {
	text, failed := in.readText(p, at)
	if failed != nil {
		return failed
	}
	return in.ok(&List{Elems: textLines(text)})
}

// readText reads the UTF-8 text of the file at p["path"], or gives the Err
// to return instead.
func (in *Interp) readText(p map[string]Value, at ir.Pos) (string, Value) {
	path := show(p["path"])
	var data []byte
	var err error
	in.blocking(func(context.Context) { data, err = os.ReadFile(in.path(p["path"], at)) })
	if err != nil {
		return "", in.errV(in.fsError(path, err, at))
	}
	if !utf8.Valid(data) {
		return "", in.errV(in.fsError(path, errors.New("not UTF-8 text"), at))
	}
	return string(data), nil
}

func (in *Interp) fsWrite(p map[string]Value, flags int, at ir.Pos) Value {
	path := show(p["path"])
	text, _ := p["text"].(string)
	if b, ok := p["bytes"]; ok {
		text = string(in.byteSlice(b, at))
	}
	var err error
	in.blocking(func(context.Context) {
		var f *os.File
		f, err = os.OpenFile(in.path(p["path"], at), flags, 0o644)
		if err != nil {
			return
		}
		_, err = f.WriteString(text)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	})
	return in.fsResult(path, err, ir.Unit{}, at)
}

func hostList(in *Interp, p map[string]Value, at ir.Pos) Value {
	path := show(p["dir"])
	entries, err := os.ReadDir(in.path(p["dir"], at))
	if err != nil {
		return in.errV(in.fsError(path, err, at))
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	sort.Strings(names)
	return in.ok(&List{Elems: toValues(names)})
}

// hostRun is process.run: any exit status is Ok; Err means the program
// didn't start.
func hostRun(in *Interp, p map[string]Value, at ir.Pos) Value {
	program, _ := p["program"].(string)
	var args []string
	for _, a := range in.elems(p["args"], at) {
		args = append(args, show(a))
	}
	if v, ok := in.fakeRun(p, program, args, at); ok {
		return v
	}
	input, _ := p["input"].(string)
	dir := in.workDir(at)
	if d, ok := p["dir"].(*Variant); ok && d.V == in.pre.some {
		dir = in.path(d.Vals[0], at)
	}
	var stdout, stderr bytes.Buffer
	var err error
	in.blocking(func(ctx context.Context) {
		cmd := exec.CommandContext(ctx, program, args...)
		cmd.Dir = dir
		cmd.Stdin = strings.NewReader(input)
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err = cmd.Run()
	})
	status := 0
	var exit *exec.ExitError
	switch {
	case errors.As(err, &exit):
		status = exit.ExitCode()
	case err != nil:
		t := in.stdModule("process", at).Types["ProcessError"]
		return in.errV(&Variant{V: t.Variants[0], Vals: []Value{program, err.Error()}})
	}
	return in.ok(&Record{
		Type:  in.stdModule("process", at).Types["Output"],
		Names: []string{"status", "stdout", "stderr"},
		Vals:  []Value{int64(status), strings.ToValidUTF8(stdout.String(), "�"), strings.ToValidUTF8(stderr.String(), "�")},
	})
}

func hostRandomInt(in *Interp, p map[string]Value, at ir.Pos) Value {
	r, ok := p["range"].(*RangeVal)
	if !ok {
		in.faultf(at, "random.int takes a range of Ints: random.int(1..=6)")
	}
	lo, ok1 := r.Start.(int64)
	hi, ok2 := r.End.(int64)
	if !ok1 || !ok2 {
		in.faultf(at, "random.int takes a range of Ints that fit in 64 bits")
	}
	if !r.Inclusive {
		hi--
	}
	if hi < lo {
		return in.none()
	}
	span := uint64(hi - lo)
	if span == ^uint64(0) {
		return in.some(int64(in.rt.random().Uint64()))
	}
	return in.some(lo + int64(in.rt.random().Uint64N(span+1)))
}

// random is the program's source of random numbers: a fixed seed in tests,
// and the system's otherwise.
func (rt *runtime) random() *rand.Rand {
	if rt.rand == nil {
		if rt.sandbox {
			rt.rand = rand.New(rand.NewPCG(1, 2))
		} else {
			rt.rand = rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
		}
	}
	return rt.rand
}
