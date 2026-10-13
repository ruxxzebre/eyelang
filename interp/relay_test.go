package interp

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// askContract has an event that another program takes: Notify's answered.
const askContract = `
pub type Answered = { question: String, choice: String, pressedBy: String }

pub type Asked = | Asked(String)

@port(4200)
@http(names: snake)
@auth(bearer: "ask-token")
contract Ask {
  @http("POST /v1/questions/:question/press")
  @key(required)
  command press(question: String, choice: String, by: String) -> Asked

  @key(question)
  event answered: Answered on "ask.answers"
}
`

// askTaker takes Ask's answered from another program: a job starts on each
// answer, and another waits for one with next!.
const askTaker = `
import ask

@on(event(ask.answered))
job record(a: ask.Answered) { print("answered {a.question}: {a.choice} by {a.pressedBy}") }

job await(q: String) {
  match next!(ask.answered, where: fn a: a.question == q, within: 1 h) {
    Some(a) => print("awaited {q}: {a.choice}")
    None => print("awaited {q}: none")
  }
}

pub fn main() uses {start await} {
  let _ = await.start("q-2", key: "await-q-2")
}
`

// askServer serves Ask: a press publishes answered, which goes to the
// programs ASK_EVENTS_URL lists.
const askServer = `
import ask.{Asked}

impl ask.Ask {
  fn press(question, choice, by, key) {
    ask.answered.publish({ question, choice, pressedBy: by })
    Asked(question)
  }
}
`

// runUntilLine runs prog under Run until ctx ends, and waits for a line of
// its output that re matches, returning the first group.
func runUntilLine(t *testing.T, ctx context.Context, opts RunOptions, files map[string]string, re string) (string, *lockedBuffer) {
	t.Helper()
	prog := loadFiles(t, files)
	ctx, cancel := context.WithCancel(ctx)
	out := &lockedBuffer{}
	opts.Stdout, opts.Stderr = out, out
	done := make(chan int)
	go func() { done <- Run(ctx, prog, opts) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Errorf("Run didn't stop:\n%s", out)
		}
	})
	line := regexp.MustCompile(re)
	var got string
	waitFor(t, 10*time.Second, "the line "+re, func() bool {
		if m := line.FindStringSubmatch(out.String()); m != nil {
			got = m[1]
			return true
		}
		return false
	})
	return got, out
}

func post(t *testing.T, url, auth, body string) shopReply {
	t.Helper()
	h := map[string]string{"Content-Type": "application/json"}
	if auth != "" {
		h["Authorization"] = auth
	}
	return shopRequest(t, http.MethodPost, url, h, body)
}

// A program takes another service's event as a POST under the contract's
// @auth: it starts the @on job and wakes the run waiting in next!, a repeat
// of its key starts nothing, and a request that isn't the event is refused.
func TestTakeEventOverHTTP(t *testing.T) {
	t.Setenv("EYE_EVENTS_PORT", "0")
	t.Setenv("ASK_TOKEN", "s3cret")
	port, out := runUntilLine(t, context.Background(), RunOptions{},
		map[string]string{"ask/ask.eye": askContract, "main.eye": askTaker},
		`eye run: taking ask\.answered on :(\d+)`)
	url := "http://127.0.0.1:" + port + "/events/ask.answers"
	const auth = "Bearer s3cret"

	for _, c := range []struct {
		name, method, url, auth, body string
		status                        int
		says                          string
	}{
		{"no secret", "POST", url, "", `{}`, 401, "ask.answered takes only events with Bearer and the secret in the Authorization header"},
		{"wrong secret", "POST", url, "Bearer guess", `{}`, 401, "takes only events"},
		{"not JSON", "POST", url, auth, `{`, 400, "the body isn't JSON"},
		{"empty", "POST", url, auth, ``, 400, "the body is empty"},
		{"wrong shape", "POST", url, auth, `{"question": 1, "choice": "yes", "pressed_by": "ann"}`, 400, "the body isn't ask.answered's value"},
		{"names not snake", "POST", url, auth, `{"question": "q-9", "choice": "yes", "pressedBy": "ann"}`, 400, "pressed_by"},
		{"another method", "GET", url, auth, ``, 405, "takes POST, not GET"},
		{"another topic", "POST", "http://127.0.0.1:" + port + "/events/ask.other", auth, `{}`, 404, "it takes POST /events/ask.answers"},
	} {
		r := shopRequest(t, c.method, c.url, map[string]string{"Authorization": c.auth}, c.body)
		if r.status != c.status || !strings.Contains(r.body, c.says) {
			t.Errorf("%s: %d %s, want %d with %q", c.name, r.status, r.body, c.status, c.says)
		}
	}

	if r := post(t, url, auth, `{"question": "q-1", "choice": "yes", "pressed_by": "ann"}`); r.status != 202 || r.body != "" {
		t.Fatalf("an event = %d %q, want 202 and no body", r.status, r.body)
	}
	waitFor(t, 10*time.Second, "the run the event starts", func() bool {
		return strings.Contains(out.String(), "answered q-1: yes by ann\n")
	})
	// The same key again is taken, and starts nothing.
	if r := post(t, url, auth, `{"question": "q-1", "choice": "no", "pressed_by": "ann"}`); r.status != 202 {
		t.Fatalf("a repeat = %d %s, want 202", r.status, r.body)
	}
	if r := post(t, url, auth, `{"question": "q-2", "choice": "later", "pressed_by": "bo"}`); r.status != 202 {
		t.Fatalf("a second event = %d %s, want 202", r.status, r.body)
	}
	waitFor(t, 10*time.Second, "the waiting run to wake", func() bool {
		s := out.String()
		return strings.Contains(s, "awaited q-2: later\n") && strings.Contains(s, "answered q-2: later by bo\n")
	})
	if s := out.String(); strings.Count(s, "answered q-1") != 1 || strings.Contains(s, "q-1: no") {
		t.Errorf("the repeat started a run:\n%s", s)
	}
}

// Without a port to take them on, eye run says that the events have no way
// in; and it won't take them without the secret their contract's @auth names.
func TestTakeEventNeedsAPortAndItsSecret(t *testing.T) {
	t.Setenv("ASK_TOKEN", "s3cret")
	files := map[string]string{"ask/ask.eye": askContract, "main.eye": askTaker}
	runUntilLine(t, context.Background(), RunOptions{}, files,
		`(eye run: ask\.answered has no way in: set EYE_EVENTS_PORT to the port its publisher sends it to)`)

	t.Setenv("EYE_EVENTS_PORT", "0")
	t.Setenv("ASK_TOKEN", "") // restored when the test ends
	_ = os.Unsetenv("ASK_TOKEN")
	t.Setenv("EYE_SECRETS", t.TempDir())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out := &lockedBuffer{}
	if code := Run(ctx, loadFiles(t, files), RunOptions{Stdout: out, Stderr: out}); code != 1 ||
		!strings.Contains(out.String(), `ask.answered needs the secret "ask-token" to check its publisher's @auth: set ASK_TOKEN`) {
		t.Errorf("Run = %d:\n%s", code, out)
	}

	// A contract without @auth would let anyone publish its events.
	t.Setenv("ASK_TOKEN", "s3cret")
	open := map[string]string{"ask/ask.eye": strings.Replace(askContract, "@auth(bearer: \"ask-token\")\n", "", 1), "main.eye": askTaker}
	_, out = runUntilLine(t, ctx, RunOptions{}, open,
		`(eye run: ask\.answered has no way in: contract Ask has no @auth, so anyone could publish it)`)
	if strings.Contains(out.String(), "taking") {
		t.Errorf("events without @auth are taken:\n%s", out)
	}

	t.Setenv("EYE_EVENTS_PORT", "a lot")
	out = &lockedBuffer{}
	if code := Run(ctx, loadFiles(t, files), RunOptions{Stdout: out, Stderr: out}); code != 1 ||
		!strings.Contains(out.String(), `EYE_EVENTS_PORT is "a lot"; it takes a port from 0 to 65535`) {
		t.Errorf("Run = %d:\n%s", code, out)
	}
}

// subscriber is a stand-in for a program that takes events: it answers
// with the statuses given, in turn, then 202, and keeps what it was sent.
type subscriber struct {
	mu       sync.Mutex
	statuses []int
	got      []string // "METHOD path auth body"
}

func (s *subscriber) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	s.mu.Lock()
	s.got = append(s.got, r.Method+" "+r.URL.Path+" "+r.Header.Get("Authorization")+" "+strings.TrimSpace(string(body)))
	status := http.StatusAccepted
	if len(s.statuses) > 0 {
		status, s.statuses = s.statuses[0], s.statuses[1:]
	}
	s.mu.Unlock()
	w.WriteHeader(status)
}

func (s *subscriber) requests() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.got...)
}

// A program sends the events of a contract it serves to each subscriber in
// <MODULE>_EVENTS_URL, with the contract's @auth, after the commit that
// publishes them, again after a 5xx, and drops one a subscriber refuses.
func TestSendEvents(t *testing.T) {
	flaky := &subscriber{statuses: []int{503}}
	refusing := &subscriber{statuses: []int{404}}
	a, b := httptest.NewServer(flaky), httptest.NewServer(refusing)
	defer a.Close()
	defer b.Close()
	t.Setenv("ASK_PORT", "0")
	t.Setenv("ASK_TOKEN", "s3cret")
	t.Setenv("ASK_EVENTS_URL", a.URL+"/, "+b.URL)
	port, out := runUntilLine(t, context.Background(), RunOptions{},
		map[string]string{"ask/ask.eye": askContract, "main.eye": askServer},
		`eye run: serving ask\.Ask on :(\d+)`)
	r := shopRequest(t, "POST", "http://127.0.0.1:"+port+"/v1/questions/q-1/press",
		map[string]string{"Authorization": "Bearer s3cret", "Idempotency-Key": "p-1"}, `{"choice": "yes", "by": "ann"}`)
	if r.status != 200 {
		t.Fatalf("press = %d %s", r.status, r.body)
	}
	want := `POST /events/ask.answers Bearer s3cret {"choice":"yes","pressed_by":"ann","question":"q-1"}`
	waitFor(t, 10*time.Second, "the event to be sent again after a 503", func() bool {
		return len(flaky.requests()) == 2
	})
	for _, got := range flaky.requests() {
		if got != want {
			t.Errorf("sent %s, want %s", got, want)
		}
	}
	waitFor(t, 10*time.Second, "the refusal to be logged", func() bool {
		return strings.Contains(out.String(), "eye run: ask.answered wasn't delivered to "+b.URL+", and is dropped: it was refused with status 404")
	})
	time.Sleep(700 * time.Millisecond) // longer than the first retry's wait
	if got := refusing.requests(); len(got) != 1 {
		t.Errorf("the refused event was sent %d times: %v", len(got), got)
	}
	if got := flaky.requests(); len(got) != 2 {
		t.Errorf("the taken event was sent %d times: %v", len(got), got)
	}
}

// A delivery is saved with the commit that publishes the event, so one that
// wasn't taken before eye run stopped is sent after it starts again.
func TestSendEventsAfterARestart(t *testing.T) {
	t.Setenv("ASK_PORT", "0")
	t.Setenv("ASK_TOKEN", "s3cret")
	down := httptest.NewServer(http.NotFoundHandler())
	downURL := down.URL
	down.Close() // nothing listens there now
	t.Setenv("ASK_EVENTS_URL", downURL)
	journal := filepath.Join(t.TempDir(), "journal")
	files := map[string]string{"ask/ask.eye": askContract, "main.eye": askServer}

	start := func() (context.CancelFunc, chan int, string, *lockedBuffer) {
		j, err := OpenJournal(journal)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		out := &lockedBuffer{}
		done := make(chan int)
		prog := loadFiles(t, files)
		go func() {
			code := Run(ctx, prog, RunOptions{Stdout: out, Stderr: out, Store: j})
			j.Close()
			done <- code
		}()
		serving := regexp.MustCompile(`eye run: serving ask\.Ask on :(\d+)`)
		var port string
		waitFor(t, 10*time.Second, "the server to listen", func() bool {
			if m := serving.FindStringSubmatch(out.String()); m != nil {
				port = m[1]
			}
			return port != ""
		})
		return cancel, done, port, out
	}
	stop := func(cancel context.CancelFunc, done chan int, out *lockedBuffer) {
		cancel()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatalf("Run didn't stop:\n%s", out)
		}
	}

	cancel, done, port, out := start()
	r := shopRequest(t, "POST", "http://127.0.0.1:"+port+"/v1/questions/q-1/press",
		map[string]string{"Authorization": "Bearer s3cret", "Idempotency-Key": "p-1"}, `{"choice": "yes", "by": "ann"}`)
	if r.status != 200 {
		t.Fatalf("press = %d %s", r.status, r.body)
	}
	time.Sleep(300 * time.Millisecond) // the first try fails
	stop(cancel, done, out)

	// The subscriber is up where the delivery was saved to, and the
	// variable has moved on: the delivery goes where it was saved to.
	t.Setenv("ASK_EVENTS_URL", "http://127.0.0.1:1")
	up := &subscriber{}
	srv := httptest.NewUnstartedServer(up)
	l, err := net.Listen("tcp", strings.TrimPrefix(downURL, "http://"))
	if err != nil {
		t.Skipf("the old port was taken meanwhile: %v", err)
	}
	srv.Listener = l
	srv.Start()
	defer srv.Close()
	cancel, done, _, out = start()
	waitFor(t, 10*time.Second, "the saved delivery to be sent", func() bool { return len(up.requests()) == 1 })
	stop(cancel, done, out)
	want := `POST /events/ask.answers Bearer s3cret {"choice":"yes","pressed_by":"ann","question":"q-1"}`
	if got := up.requests(); got[0] != want {
		t.Errorf("sent %s, want %s", got[0], want)
	}

	// Done, it isn't sent again.
	cancel, done, _, out = start()
	time.Sleep(300 * time.Millisecond)
	stop(cancel, done, out)
	if got := up.requests(); len(got) != 1 {
		t.Errorf("sent %d times: %v", len(got), got)
	}
}

// An event crosses between two eye runs: the program that serves Ask
// publishes answered, and the program that consumes it runs its jobs.
func TestEventCrossesBetweenPrograms(t *testing.T) {
	t.Setenv("EYE_EVENTS_PORT", "0")
	t.Setenv("ASK_PORT", "0")
	t.Setenv("ASK_TOKEN", "s3cret")
	takerPort, takerOut := runUntilLine(t, context.Background(), RunOptions{},
		map[string]string{"ask/ask.eye": askContract, "main.eye": askTaker},
		`eye run: taking ask\.answered on :(\d+)`)
	t.Setenv("ASK_EVENTS_URL", "http://127.0.0.1:"+takerPort)
	port, serverOut := runUntilLine(t, context.Background(), RunOptions{},
		map[string]string{"ask/ask.eye": askContract, "main.eye": askServer},
		`eye run: serving ask\.Ask on :(\d+)`)
	if strings.Contains(serverOut.String(), "taking") {
		t.Errorf("the server takes its own contract's events:\n%s", serverOut)
	}
	for range 2 { // pressed twice: the key starts one run
		r := shopRequest(t, "POST", "http://127.0.0.1:"+port+"/v1/questions/q-2/press",
			map[string]string{"Authorization": "Bearer s3cret", "Idempotency-Key": "p-2"}, `{"choice": "yes", "by": "ann"}`)
		if r.status != 200 {
			t.Fatalf("press = %d %s", r.status, r.body)
		}
	}
	waitFor(t, 10*time.Second, "the taker's runs", func() bool {
		s := takerOut.String()
		return strings.Contains(s, "answered q-2: yes by ann\n") && strings.Contains(s, "awaited q-2: yes\n")
	})
	time.Sleep(200 * time.Millisecond)
	if n := strings.Count(takerOut.String(), "answered q-2"); n != 1 {
		t.Errorf("the event started %d runs:\n%s", n, takerOut)
	}
}
