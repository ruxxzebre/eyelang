package main

import (
	"bytes"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"eyelang/check"
	"eyelang/interp"
)

// The exit test of 6.13 (ROADMAP.md, #81): two Eyelang programs under eye
// run, one serving the lab's Notify contract on its port and one calling
// notify.send!. The call reaches the notifier's impl; a repeat with the same
// key is answered without a second send; a request without Notify's token
// is refused.
func TestServeNotifyBetweenPrograms(t *testing.T) {
	t.Setenv("NOTIFY_TOKEN", "notify-secret")
	t.Setenv("NOTIFY_PORT", "0") // any free port; the server names it
	t.Setenv("NOTIFY_URL", "")

	// The server: the lab's notifier, alone.
	server := labCopy(t, "server", map[string]string{"lab.eye": "import notifier\n"})
	srv, srvOut := eyeLockedProcess(t, "run", server)
	defer stopProcess(t, srv, srvOut, 0)
	serving := regexp.MustCompile(`eye run: serving notify\.Notify on :(\d+)`)
	var port string
	waitUntil(t, "the notifier listens", srvOut, func() bool {
		if m := serving.FindStringSubmatch(srvOut.String()); m != nil {
			port = m[1]
		}
		return port != ""
	})
	url := "http://127.0.0.1:" + port

	// The caller: a job in the digest, which Notify's @allow lists, sends
	// one message twice with one key.
	t.Setenv("NOTIFY_URL", url)
	callerFiles := map[string]string{
		"lab.eye": "import digest\n\npub fn main() uses {start sendTwice} {\n  digest.sendTwice.start(key: \"exit-6.13\")\n}\n",
		"digest/sendtwice.eye": `import home.{operator}
import notify

pub job sendTwice() -> String uses {net Notify.send, wait} {
  let first = notify.send!(event: "digest.ireland", title: "Hello", body: "From the digest", to: Some([operator]), key: "exit-6.13").reconcile!()
  let again = notify.send!(event: "digest.ireland", title: "Hello", body: "From the digest", to: Some([operator]), key: "exit-6.13").reconcile!()
  "{first}; {again}"
}
`,
	}
	caller := labCopy(t, "caller", callerFiles)
	call, callOut := eyeLockedProcess(t, "run", caller)
	sent := waitForRun(t, caller, "digest.sendTwice", callOut)
	stopProcess(t, call, callOut, 0)
	want := `"Accepted(id: \"exit-6.13\", deliveries: 1, deduped: false); Accepted(id: \"exit-6.13\", deliveries: 1, deduped: true)"`
	if sent.Status != "done" || sent.Result != want {
		t.Errorf("sendTwice is %s: %s%s\nwant %s", sent.Status, sent.Result, sent.Fault, want)
	}

	// The notifier queued the message once: one Message, and one deliver
	// run for its one recipient.
	deliveries := 0
	waitUntil(t, "the notifier's deliver run is saved", srvOut, func() bool {
		deliveries = 0
		for _, r := range readRuns(t, server) {
			if r.Job == "notifier.deliver" {
				deliveries++
			}
		}
		return deliveries > 0
	})
	if deliveries != 1 {
		t.Errorf("the notifier started %d deliver runs, want 1", deliveries)
	}

	// A request without Notify's token is refused before it reaches code.
	body := `{"event": "digest.ireland", "title": "x", "body": "y", "to": ["operator"]}`
	for _, token := range []string{"", "Bearer guess"} {
		req, _ := http.NewRequest("POST", url+"/v1/send", strings.NewReader(body))
		req.Header.Set("X-Actions-Idempotency-Key", "intruder")
		if token != "" {
			req.Header.Set("Authorization", token)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("a send with Authorization %q answered %d, want 401", token, resp.StatusCode)
		}
	}

	// A caller with the wrong token gets the refusal as a fault of its run.
	t.Setenv("NOTIFY_TOKEN", "guess")
	intruder := labCopy(t, "intruder", callerFiles)
	refused, refusedOut := eyeLockedProcess(t, "run", intruder)
	run := waitForRun(t, intruder, "digest.sendTwice", refusedOut)
	stopProcess(t, refused, refusedOut, 1) // its run faulted
	if run.Status != "failed" || !strings.Contains(run.Fault, "refused with status 401") {
		t.Errorf("sendTwice with a wrong token is %s: %s%s", run.Status, run.Result, run.Fault)
	}
	for _, r := range readRuns(t, server) {
		if r.Job == "notifier.deliver" && r.Args[0] != `"exit-6.13"` {
			t.Errorf("a refused request started a delivery: %v", r.Args)
		}
	}
}

// labCopy copies the lab into a directory of its own, without its journal,
// and writes files over it.
func labCopy(t *testing.T, name string, files map[string]string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	if out, err := exec.Command("cp", "-R", "../../testdata/lab", dir).CombinedOutput(); err != nil {
		t.Fatalf("copying the lab: %v\n%s", err, out)
	}
	_ = os.RemoveAll(filepath.Join(dir, ".eye"))
	for path, src := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, path)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, path), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, diags := check.Program(dir); len(diags) > 0 {
		t.Fatalf("the lab's %s copy doesn't check: %v", name, diags)
	}
	return dir
}

// lockedOutput is what a process writes, read while it writes.
type lockedOutput struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (o *lockedOutput) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.b.Write(p)
}

func (o *lockedOutput) String() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.b.String()
}

// eyeLockedProcess starts eye as a process of its own, with its output in a
// buffer the test can read while it runs.
func eyeLockedProcess(t *testing.T, args ...string) (*exec.Cmd, *lockedOutput) {
	t.Helper()
	cmd := exec.Command(os.Args[0], args...)
	cmd.Env = append(os.Environ(), "EYE_AS_COMMAND=1")
	out := &lockedOutput{}
	cmd.Stdout, cmd.Stderr = out, out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	return cmd, out
}

// stopProcess stops eye run as Ctrl-C does, once; it must exit with code.
func stopProcess(t *testing.T, cmd *exec.Cmd, out *lockedOutput, code int) {
	t.Helper()
	if cmd.ProcessState != nil {
		return
	}
	_ = cmd.Process.Signal(os.Interrupt)
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
		if got := cmd.ProcessState.ExitCode(); got != code {
			t.Errorf("eye run on Ctrl-C exited with %d, want %d:\n%s", got, code, out)
		}
	case <-time.After(20 * time.Second):
		_ = cmd.Process.Kill()
		<-done
		t.Errorf("eye run didn't stop on Ctrl-C:\n%s", out)
	}
}

func waitUntil(t *testing.T, what string, out interface{ String() string }, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(60 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if cond() {
			return
		}
	}
	t.Fatalf("timed out waiting until %s; eye run said:\n%s", what, out.String())
}

// readRuns reads the runs in a program's journal, while eye run writes it.
func readRuns(t *testing.T, dir string) []interp.RunInfo {
	t.Helper()
	prog, diags := check.Program(dir)
	if len(diags) > 0 {
		t.Fatal(diags)
	}
	store, err := interp.ReadJournal(filepath.Join(dir, ".eye", "journal"))
	if err != nil {
		return nil
	}
	return interp.Runs(prog, store)
}

// waitForRun waits until the program's run of job has finished, and returns it.
func waitForRun(t *testing.T, dir, job string, out interface{ String() string }) interp.RunInfo {
	t.Helper()
	var found interp.RunInfo
	waitUntil(t, job+" finishes", out, func() bool {
		for _, r := range readRuns(t, dir) {
			if r.Job == job {
				found = r
			}
		}
		return found.Status == "done" || found.Status == "failed"
	})
	return found
}
