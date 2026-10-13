package cgen

import (
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"eyelang/interp"
	"eyelang/ir"
)

// served maps each test binary build makes to its program, so that what
// runs the binary can answer its host calls.
var served sync.Map

// serveHosts answers the host calls of the binary exe that cmd runs, such
// as http.get, with the interpreter's host functions, over the two file
// descriptors runtime/host.c reads EYE_HOST_FDS for: what the Go runner
// will do through cgo (ROADMAP 7.6). Call it before cmd.Start, and the
// function it gives after cmd.Wait.
func serveHosts(t *testing.T, cmd *exec.Cmd, exe string) func() {
	t.Helper()
	p, ok := served.Load(exe)
	if !ok {
		return func() {}
	}
	prog := p.(*ir.Program)
	reqR, reqW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	ansR, ansW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.ExtraFiles = []*os.File{ansR, reqW} // fds 3 and 4
	if cmd.Env == nil {
		cmd.Env = os.Environ()
	}
	cmd.Env = append(cmd.Env, "EYE_HOST_FDS=3,4")
	done := make(chan struct{})
	go func() {
		defer close(done)
		in := interp.New(prog, io.Discard, io.Discard)
		for {
			name, err := readFrame(reqR, 4)
			if err != nil {
				return // the binary has exited
			}
			args, err := readFrame(reqR, 8)
			if err != nil {
				return
			}
			status, answer := byte(0), []byte(nil)
			if answer, err = in.HostCall(string(name), args); err != nil {
				status, answer = 1, []byte(err.Error())
			}
			head := make([]byte, 9)
			head[0] = status
			binary.LittleEndian.PutUint64(head[1:], uint64(len(answer)))
			if _, err := ansW.Write(append(head, answer...)); err != nil {
				return
			}
		}
	}()
	return func() {
		// With the binary gone and these closed, nothing holds the
		// requests' write end, and the server reads their end.
		ansR.Close()
		reqW.Close()
		<-done
		reqR.Close()
		ansW.Close()
	}
}

// readFrame reads a length of n bytes, little-endian, and that many bytes.
func readFrame(r io.Reader, n int) ([]byte, error) {
	head := make([]byte, 8)
	if _, err := io.ReadFull(r, head[:n]); err != nil {
		return nil, err
	}
	size := binary.LittleEndian.Uint64(head)
	if size > 1<<30 {
		return nil, fmt.Errorf("a frame of %d bytes", size)
	}
	data := make([]byte, size)
	_, err := io.ReadFull(r, data)
	return data, err
}

// TestMain serves the pages cgen/testdata/hostcalls gets, at the address in
// EYE_CGEN_HTTP, while the package's tests run, so that the interpreter and
// the binaries get the same pages.
func TestMain(m *testing.M) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/hello":
			w.Header().Set("X-Answer", "42")
			fmt.Fprintf(w, "hello, %s", r.Header.Get("Accept"))
		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, "no such page")
		}
	}))
	os.Setenv("EYE_CGEN_HTTP", srv.URL)
	code := m.Run()
	srv.Close()
	os.Exit(code)
}

// TestHostCallsServed checks that cgen/testdata/hostcalls gets the local
// server's pages natively, so that the cross-check, which compares its
// native output with the interpreter's, compares pages and not two
// failures.
func TestHostCallsServed(t *testing.T) {
	needZig(t)
	if HostTarget() == "" {
		t.Skip("eye build doesn't target this machine")
	}
	got := native(t, build(t, load(t, "testdata/hostcalls"), "", true, true))
	for _, want := range []string{"200 hello,  x-answer=42", "200 hello, text/plain x-answer=42", "404 no such page x-answer=none"} {
		if !strings.Contains(got, want) {
			t.Errorf("the binary didn't print %q:\n%s", want, got)
		}
	}
}
