package host_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"eyelang/host"
)

type counter struct{ n int }

type other struct{}

// register provides every host function testdata/desk declares.
func register(t *testing.T, p *host.Program, shown *[]string) {
	t.Helper()
	hosts := map[string]any{
		"lab.desk": func(text string) error {
			if text == "" {
				return errors.New("the desk shows no empty line")
			}
			*shown = append(*shown, text)
			return nil
		},
		"lab.add":     func(a, b int32) int32 { return a + b },
		"lab.counter": func(start int) *counter { return &counter{n: start} },
		"lab.bump":    func(c *counter, by int) int { c.n += by; return c.n },
		"lab.sum": func(ctx context.Context, data []byte) (uint64, error) {
			if ctx.Err() != nil {
				return 0, ctx.Err()
			}
			if len(data) == 0 {
				return 0, errors.New("nothing to sum")
			}
			var n uint64
			for _, b := range data {
				n += uint64(b)
			}
			return n, nil
		},
	}
	for name, fn := range hosts {
		if err := p.Register(name, fn); err != nil {
			t.Fatal(err)
		}
	}
}

func load(t *testing.T) *host.Program {
	t.Helper()
	p, err := host.Load("testdata/desk")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// TestEmbedHostFunctions embeds the interpreter: a Go program registers
// host functions, and the program's main and tests call them.
func TestEmbedHostFunctions(t *testing.T) {
	p := load(t)
	var shown []string
	register(t, p, &shown)
	var out, errs bytes.Buffer
	code := p.Run(context.Background(), host.RunOptions{Args: []string{"desk"}, Stdout: &out, Stderr: &errs})
	if code != 1 {
		t.Errorf("exit status %d, want 1 from main's Err", code)
	}
	if got, want := out.String(), "5 41 42\n294\n"; got != want {
		t.Errorf("printed %q, want %q", got, want)
	}
	if got := errs.String(); !strings.Contains(got, "the desk shows no empty line\n") {
		t.Errorf("stderr is %q, want main's Err from the host function", got)
	}
	if len(shown) != 1 || shown[0] != "hello from Eyelang" {
		t.Errorf("the desk showed %q", shown)
	}
	results := p.Test("")
	if len(results) != 2 {
		t.Fatalf("ran %d tests, want 2", len(results))
	}
	for _, r := range results {
		if r.Failure != "" {
			t.Errorf("%s: %s", r.Name, r.Failure)
		}
	}
	if len(shown) != 2 || shown[1] != "from a job" {
		t.Errorf("the desk showed %q, want the job's line last", shown)
	}
}

// failure is the failure of the test named name.
func failure(t *testing.T, results []host.TestResult, name string) string {
	t.Helper()
	for _, r := range results {
		if r.Name == name {
			return r.Failure
		}
	}
	t.Fatalf("no test named %q", name)
	return ""
}

// TestEmbedWrongSignature checks that Register reports a Go function that
// doesn't match its declaration, rather than a call crashing later.
func TestEmbedWrongSignature(t *testing.T) {
	cases := []struct {
		name string
		fn   any
		want string
	}{
		{"lab.add", func(a, b int64) int64 { return a + b },
			`@host("lab.add") is add(a: Int32, b: Int32) -> Int32 at testdata/desk/main.eye:8:1: parameter a is Int32, which Go holds as int32, and the Go function's parameter 1 is int64`},
		{"lab.add", func(a int32) int32 { return a },
			`@host("lab.add") is add(a: Int32, b: Int32) -> Int32 at testdata/desk/main.eye:8:1, which takes 2 arguments, and the Go function takes 1`},
		{"lab.desk", func(string) {},
			`@host("lab.desk") is showOnDesk(text: String) -> Result((), String) at testdata/desk/main.eye:5:1, which a Go function returns as (error), and this one returns ()`},
		{"lab.sum", func([]byte) uint64 { return 0 },
			`@host("lab.sum") is sum(data: Bytes) -> Result(UInt64, String) at testdata/desk/main.eye:20:1, which a Go function returns as (uint64, error), and this one returns (uint64)`},
		{"lab.counter", func(int) int { return 0 },
			`@host("lab.counter") is counter(start: Int) -> Counter at testdata/desk/main.eye:14:1, which a Go function returns as (a pointer, struct, map, channel, function or interface), and this one returns (int)`},
		{"lab.bump", func(c string, by int) int { return 0 },
			`@host("lab.bump") is bump(c: Counter, by: Int) -> Int at testdata/desk/main.eye:17:1: parameter c is Counter, which Go holds as a pointer, struct, map, channel, function or interface, and the Go function's parameter 1 is string`},
		{"lab.add", func(xs ...int32) int32 { return 0 },
			`@host("lab.add") is add(a: Int32, b: Int32) -> Int32 at testdata/desk/main.eye:8:1, and the Go function is variadic`},
		{"lab.add", 42, `@host("lab.add") needs a Go function, and this is int`},
		{"lab.nowhere", func() {}, `no extern fn of the program is declared @host("lab.nowhere")`},
		{"http.get", func() {}, `@host("http.get") is the interpreter's own: register a name of the program's`},
	}
	for _, c := range cases {
		err := load(t).Register(c.name, c.fn)
		if err == nil || err.Error() != c.want {
			t.Errorf("Register(%q):\n got %v\nwant %s", c.name, err, c.want)
		}
	}
	p := load(t)
	add := func(a, b int32) int32 { return a + b }
	if err := p.Register("lab.add", add); err != nil {
		t.Fatal(err)
	}
	if err := p.Register("lab.add", add); err == nil || err.Error() != `@host("lab.add") is registered twice` {
		t.Errorf("registering twice: %v", err)
	}

	// Types no Go function can take or give.
	u, err := host.Load("testdata/unhostable")
	if err != nil {
		t.Fatal(err)
	}
	const types = "Bool, Byte, Int, Int32, Int64, UInt32, UInt64, Float, String, Bytes and extern types"
	for name, want := range map[string]string{
		"lab.count": `@host("lab.count") is count(xs: List(Int)) -> Int at testdata/unhostable/main.eye:3:1: parameter xs is List(Int), which no Go type stands for: a host function takes ` + types,
		"lab.first": `@host("lab.first") is first(xs: String) -> Option(String) at testdata/unhostable/main.eye:6:1, and no Go function can return Option(String): a host function returns ` + types + `, (), or Result(T, String) as (T, error)`,
		"lab.ask":   `@host("lab.ask") is ask(q: String) -> Result(String, Int) at testdata/unhostable/main.eye:9:1, and no Go function can return Result(String, Int): a host function returns ` + types + `, (), or Result(T, String) as (T, error)`,
	} {
		if err := u.Register(name, func() {}); err == nil || err.Error() != want {
			t.Errorf("Register(%q):\n got %v\nwant %s", name, err, want)
		}
	}
}

// TestEmbedMissingHost checks that Run reports host functions nothing
// registered before main starts, and that a test calling one fails at the
// call.
func TestEmbedMissingHost(t *testing.T) {
	p := load(t)
	if err := p.Register("lab.add", func(a, b int32) int32 { return a + b }); err != nil {
		t.Fatal(err)
	}
	var out, errs bytes.Buffer
	if code := p.Run(context.Background(), host.RunOptions{Stdout: &out, Stderr: &errs}); code != 1 {
		t.Errorf("exit status %d, want 1", code)
	}
	want := `testdata/desk/main.eye:5:1: showOnDesk is @host("lab.desk"), and nothing registered a function of that name: eye provides only the standard library's, so run the program from the Go program that embeds it (package eyelang/host)`
	if !strings.HasPrefix(errs.String(), want+"\n") || strings.Count(errs.String(), "\n") != 4 {
		t.Errorf("stderr:\n%s\nwant four lines, the first\n%s", errs.String(), want)
	}
	if out.Len() > 0 {
		t.Errorf("main ran and printed %q", out.String())
	}
	r := p.Test("")
	if f := failure(t, r, "host functions answer in tests"); !strings.Contains(f, `testdata/desk/main.eye:38:10: sum is @host("lab.sum"), and nothing registered`) {
		t.Errorf("the test failed with %q", f)
	}
	if f := failure(t, r, "a host function in a job is a step"); !strings.Contains(f, `showOnDesk is @host("lab.desk"), and nothing registered`) {
		t.Errorf("the job's test failed with %q", f)
	}
}

// TestEmbedFaults checks what a host function's panic, and a foreign value
// passed to a host function that takes another Go type, do: each is a fault
// at the call, not a crash.
func TestEmbedFaults(t *testing.T) {
	q := load(t)
	hosts := map[string]any{
		"lab.desk":    func(string) error { panic("the desk is unplugged") },
		"lab.add":     func(a, b int32) int32 { return a + b },
		"lab.counter": func(start int) *counter { return &counter{n: start} },
		"lab.bump":    func(c *other, by int) int { return 0 },
		"lab.sum":     func([]byte) (uint64, error) { return 0, errors.New("nothing to sum") },
	}
	for name, fn := range hosts {
		if err := q.Register(name, fn); err != nil {
			t.Fatal(err)
		}
	}
	var errs bytes.Buffer
	if code := q.Run(context.Background(), host.RunOptions{Stdout: &bytes.Buffer{}, Stderr: &errs}); code != 1 {
		t.Errorf("exit status %d, want 1", code)
	}
	if want := `testdata/desk/main.eye:24:3: @host("lab.desk") panicked: the desk is unplugged`; !strings.Contains(errs.String(), want) {
		t.Errorf("stderr is %q, want %q", errs.String(), want)
	}
	r := q.Test("")
	want := `@host("lab.bump") takes *host_test.other as its c, and this Counter holds a *host_test.counter, which another host function made`
	if f := failure(t, r, "host functions answer in tests"); !strings.Contains(f, want) {
		t.Errorf("the test failed with %q, want %q", f, want)
	}
}

// TestEmbedCheckError checks that a program that doesn't check is a
// CheckError with each diagnostic.
func TestEmbedCheckError(t *testing.T) {
	_, err := host.Load(t.TempDir())
	var ce *host.CheckError
	if !errors.As(err, &ce) || len(ce.Diagnostics) == 0 || !strings.Contains(ce.Diagnostics[0], "has no .eye files") {
		t.Errorf("Load of an empty directory: %v", err)
	}
}
