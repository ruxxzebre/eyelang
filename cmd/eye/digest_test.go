package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"eyelang/check"
	"eyelang/internal/pgtest"
	"eyelang/interp"
)

// The exit test of phase 5 (ROADMAP.md, #53): the lab's morning digest runs
// under eye run on the Postgres store, against stand-ins for the services it
// calls. The runner is killed while the model writes, and the database stops
// while Notify is sent the briefing; the digest still goes out once, and the
// same program runs unchanged on the journal and in memory.
func TestMorningDigestOnPostgres(t *testing.T) {
	url, container := pgtest.Own(t)
	lab := labServices(t)
	dir := labWithMain(t)

	lab.holdModel()
	lab.holdNotify()
	first, out := eyeProcess(t, "run", "--store", url, dir)
	defer func() { _ = first.Process.Kill() }()
	lab.waitModelCalled(t, 1, out)

	// Killed in the middle of the model's answer: the call wasn't recorded,
	// so the run asks again after the restart.
	if err := first.Process.Signal(syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	_ = first.Wait()
	lab.releaseModel()

	second, out := eyeProcess(t, "run", "--store", url, dir)
	defer func() { _ = second.Process.Kill() }()
	lab.waitNotifyCalled(t, 1, out)

	// The database stops while Notify holds the send, and the reply is
	// lost: the step can't be saved, and is tried again once it is back.
	pgtest.Docker(t, "stop", container)
	lab.releaseNotify()
	time.Sleep(2 * time.Second)
	pgtest.Docker(t, "start", container)
	pgtest.Ready(t, url)

	done := waitForMorning(t, func() (interp.Store, error) { return interp.ReadPostgres(url) }, dir, out)
	if !strings.Contains(out.String(), "trying again") {
		t.Errorf("no commit failed while the database was down; eye run said:\n%s", out)
	}
	if !strings.HasPrefix(done.Result, "Ok(Accepted(") {
		t.Errorf("morning returned %s", done.Result)
	}
	if n := lab.modelCalls(); n != 2 {
		t.Errorf("the model was asked %d times, want 2: once before the kill, once after", n)
	}
	keys := lab.notifyKeys()
	if len(keys) != 2 || keys[0] != "ireland-2026-10-12" || keys[1] != keys[0] {
		t.Errorf("Notify got sends with keys %v, want the same key twice: the lost one and its retry", keys)
	}

	// The same program, unchanged, on the journal and in memory.
	for _, store := range [][]string{nil, {"--memory"}} {
		lab.reset()
		d := labWithMain(t)
		p, out := eyeProcess(t, append(append([]string{"run"}, store...), d)...)
		lab.waitNotifyCalled(t, 2, out) // the lost reply and the retry
		if store == nil {
			waitForMorning(t, func() (interp.Store, error) { return interp.ReadJournal(filepath.Join(d, ".eye", "journal")) }, d, out)
		}
		_ = p.Process.Kill()
		_ = p.Wait()
	}
}

// waitForMorning waits until the morning run is done, and returns it.
func waitForMorning(t *testing.T, read func() (interp.Store, error), dir string, out interface{ String() string }) interp.RunInfo {
	t.Helper()
	prog, diags := check.Program(dir)
	if len(diags) > 0 {
		t.Fatal(diags)
	}
	var morning interp.RunInfo
	for deadline := time.Now().Add(90 * time.Second); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		store, err := read()
		if err != nil {
			continue
		}
		for _, r := range interp.Runs(prog, store) {
			if r.Job == "digest.morning" {
				morning = r
			}
		}
		if morning.Status == "done" || morning.Status == "failed" {
			return morning
		}
	}
	t.Fatalf("the morning run is %q after 90 s; eye run said:\n%s", morning.Status, out.String())
	return morning
}

// labWithMain copies the lab into a directory of its own, with a main that
// starts the morning digest for 12 October at once rather than at 07:40.
func labWithMain(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "lab")
	if out, err := exec.Command("cp", "-R", "../../testdata/lab", dir).CombinedOutput(); err != nil {
		t.Fatalf("copying the lab: %v\n%s", err, out)
	}
	_ = os.RemoveAll(filepath.Join(dir, ".eye"))
	// morning is private to its module; only this copy makes it public, so that main can start it.
	path := filepath.Join(dir, "digest", "digest.eye")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(src), "\njob morning(") != 1 {
		t.Fatal("digest.eye no longer declares `job morning(` once; update labWithMain")
	}
	src = []byte(strings.Replace(string(src), "\njob morning(", "\npub job morning(", 1))
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatal(err)
	}
	main := "pub fn main() uses {start morning} {\n  digest.morning.start(2026-10-12, key: \"exit-test\")\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "start.eye"), []byte(main), 0o644); err != nil {
		t.Fatal(err)
	}
	// A public job states its effects, so eye fmt writes morning's clause
	// from what its body does, as it would for anyone making it public.
	var stderr strings.Builder
	if code := runFmt([]string{dir}, io.Discard, &stderr); code != 0 {
		t.Fatalf("eye fmt on the lab with a main exited %d:\n%s", code, stderr.String())
	}
	// A copy that doesn't check would only show as a run that never asks the model.
	if _, diags := check.Program(dir); len(diags) > 0 {
		t.Fatalf("the lab with a main doesn't check: %v", diags)
	}
	return dir
}

// lab holds stand-ins for the services the digest calls: Miniflux, the VPN's
// proxy, FlareSolverr, the model gateway and Notify. The model and Notify can
// be held, to stop a run in the middle of a call.
type lab struct {
	mu         sync.Mutex
	models     int
	keys       []string
	modelHeld  chan struct{}
	notifyHeld chan struct{}
}

func (l *lab) holdModel()  { l.mu.Lock(); l.modelHeld = make(chan struct{}); l.mu.Unlock() }
func (l *lab) holdNotify() { l.mu.Lock(); l.notifyHeld = make(chan struct{}); l.mu.Unlock() }

func (l *lab) releaseModel() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.modelHeld != nil {
		close(l.modelHeld)
		l.modelHeld = nil
	}
}

func (l *lab) releaseNotify() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.notifyHeld != nil {
		close(l.notifyHeld)
		l.notifyHeld = nil
	}
}

func (l *lab) reset() {
	l.releaseModel()
	l.releaseNotify()
	l.mu.Lock()
	l.models, l.keys = 0, nil
	l.mu.Unlock()
}

func (l *lab) modelCalls() int { l.mu.Lock(); defer l.mu.Unlock(); return l.models }

func (l *lab) notifyKeys() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.keys...)
}

func (l *lab) waitModelCalled(t *testing.T, n int, out interface{ String() string }) {
	t.Helper()
	l.waitUntil(t, "the model is asked", out, func() bool { return l.models >= n })
}

func (l *lab) waitNotifyCalled(t *testing.T, n int, out interface{ String() string }) {
	t.Helper()
	l.waitUntil(t, "Notify is sent the briefing", out, func() bool { return len(l.keys) >= n })
}

func (l *lab) waitUntil(t *testing.T, what string, out interface{ String() string }, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(60 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		l.mu.Lock()
		ok := cond()
		l.mu.Unlock()
		if ok {
			return
		}
	}
	t.Fatalf("timed out waiting until %s; eye run said:\n%s", what, out.String())
}

// labServices starts the stand-ins and points the lab's environment at them.
func labServices(t *testing.T) *lab {
	t.Helper()
	l := &lab{}
	serve := func(h http.HandlerFunc) string {
		s := httptest.NewServer(h)
		t.Cleanup(s.Close)
		return s.URL
	}
	miniflux := serve(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/me":
			_, _ = io.WriteString(w, `{"id": 1, "username": "digest"}`)
		case r.URL.Path == "/v1/categories":
			_, _ = io.WriteString(w, `[{"id": 4, "title": "Digest: Temporary protection"}, {"id": 5, "title": "Digest: Immigration"},
				{"id": 6, "title": "Digest: Ireland"}, {"id": 7, "title": "Digest: Dublin"}, {"id": 8, "title": "Digest: Dublin events"},
				{"id": 9, "title": "Digest: Money and housing"}, {"id": 10, "title": "Digest: Transport and weather"}]`)
		case r.URL.Path == "/v1/feeds" && r.Method == "GET":
			_, _ = io.WriteString(w, `[]`)
		case r.URL.Path == "/v1/feeds":
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"feed_id": 1}`)
		case strings.HasSuffix(r.URL.Path, "/entries"):
			_, _ = io.WriteString(w, `{"total": 1, "entries": [{"id": 9, "title": "Rent cap extended",
				"url": "https://rte.ie/rent", "published_at": "2026-10-12T05:00:00Z",
				"content": "<p>The rent cap is extended to 2027.</p>", "feed": {"title": "RTÉ"}}]}`)
		default:
			http.NotFound(w, r)
		}
	})
	proxy := serve(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no route", http.StatusBadGateway)
	})
	flare := serve(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"status": "ok", "message": "", "solution": {"url": "https://www.dublincity.ie/events",
			"status": 200, "response": "<html>Culture Night, Friday</html>"}}`)
	})
	gateway := serve(func(w http.ResponseWriter, r *http.Request) {
		l.mu.Lock()
		l.models++
		held := l.modelHeld
		l.mu.Unlock()
		if held != nil {
			select {
			case <-held:
			case <-r.Context().Done():
				return // the runner was killed while it waited
			}
		}
		briefing := `{"highlights": ["Rent cap extended to 2027"], "sections": [
			{"heading": "Temporary protection", "stories": []},
			{"heading": "Dublin", "stories": [{"headline": "Rent cap extended", "text": "It now runs to 2027.",
				"action": null, "sources": [{"title": "RTÉ", "url": "https://rte.ie/rent"}]}]},
			{"heading": "Coming up in Dublin", "stories": []}]}`
		reply, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{
			"finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": briefing}}},
			"usage": map[string]any{"prompt_tokens": 2000, "completion_tokens": 400}})
		_, _ = w.Write(reply)
	})
	notify := serve(func(w http.ResponseWriter, r *http.Request) {
		l.mu.Lock()
		l.keys = append(l.keys, r.Header.Get("X-Actions-Idempotency-Key"))
		first := len(l.keys) == 1
		held := l.notifyHeld
		l.mu.Unlock()
		if held != nil {
			<-held
		}
		if first {
			// The send arrived, and its reply is lost.
			conn, _, _ := w.(http.Hijacker).Hijack()
			conn.Close()
			return
		}
		_, _ = io.WriteString(w, `{"id": "msg-1", "deliveries": 1, "deduped": true}`)
	})
	for k, v := range map[string]string{
		"MINIFLUX_URL": miniflux, "MINIFLUX_KEY": "mf-key", "VPN_PROXY_URL": proxy,
		"FLARESOLVERR_URL": flare, "AI_URL": gateway + "/v1", "AI_KEY": "ai-secret",
		"DIGEST_MODEL": "test-model", "AI_PRICES": "test-model=1/4",
		"NOTIFY_URL": notify, "NOTIFY_TOKEN": "notify-secret",
		// The lab serves Notify too (its notifier), on any free port.
		"NOTIFY_PORT": "0",
		// eye run reads the pages without a proxy through these, which
		// refuse every request: the test reads no website.
		"HTTP_PROXY": proxy, "HTTPS_PROXY": proxy,
	} {
		t.Setenv(k, v)
	}
	return l
}
