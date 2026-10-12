package interp

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"eyelang/ir"
)

// A contract module with a command that goes over HTTP and a query that
// doesn't, and tests that use them through fakes.
var liveNotify = map[string]string{
	"notify/notify.eye": `
pub type Delivery =
  | Accepted(id: String, deliveries: Int)
  | Refused(error: String)

@auth(bearer: "notify-token")
contract Notify {
  @http("POST /v1/send", status: { 200: Accepted, 400: Refused })
  @key(required, header: "X-Key")
  command send(title: String) -> Delivery

  query who() -> String
}
`,
	"main.eye": `
import notify

job hello(title: String) {
  notify.send!(title, key: "hello-{title}")
}

job ask() {
  notify.who!()
}

test "a send goes to Notify" {
  let sent = fakes(notify, from: "recordings/")
  with sent
  hello.start("morning")
  expect(sent.calls(notify.send) == [{ title: "morning", key: "hello-morning" }])
}

test "a query without @http" {
  with fakes(notify, from: "recordings/")
  ask.start()
}
`,
}

// unsetenv unsets variables for the rest of the test, and sets them back
// after it.
func unsetenv(t *testing.T, names ...string) {
	t.Helper()
	for _, n := range names {
		t.Setenv(n, "")
		os.Unsetenv(n)
	}
}

// liveRun runs the program's tests with the modules named live.
func liveRun(t *testing.T, prog *ir.Program, record bool, filter string, names ...string) ([]TestResult, []Recorded) {
	t.Helper()
	mods, err := LiveModules(prog, names)
	if err != nil {
		t.Fatal(err)
	}
	results, written, err := RunTestsWith(prog, TestOptions{Filter: filter, Live: mods, Record: record})
	if err != nil {
		t.Fatal(err)
	}
	return results, written
}

// notifyService plays Notify: it accepts every send, and puts the token it
// was sent into the message's id, as a careless service might.
func notifyService(t *testing.T) *[]*http.Request {
	var mu sync.Mutex
	var got []*http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		got = append(got, r)
		mu.Unlock()
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		_, _ = io.WriteString(w, `{"id": "m-1 for `+token+`", "deliveries": 2}`)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("NOTIFY_URL", srv.URL)
	t.Setenv("NOTIFY_TOKEN", "notify-secret-77")
	return &got
}

func TestLiveContractSkipsWithoutAddressOrSecret(t *testing.T) {
	unsetenv(t, "NOTIFY_URL", "NOTIFY_TOKEN", "EYE_SECRETS")
	prog := loadFiles(t, liveNotify)
	results, _ := liveRun(t, prog, false, "a send", "notify")
	r := results[0]
	if r.Failure != "" || !strings.Contains(r.Skipped, "--live notify: set NOTIFY_URL, the address of its service, and NOTIFY_TOKEN, its secret \"notify-token\"") {
		t.Errorf("skipped %q, failure %q", r.Skipped, r.Failure)
	}
	// Without --live, the same test replays its recordings, and has none.
	if r := RunTests(prog, "a send")[0]; r.Skipped != "" || !strings.Contains(r.Failure, "no recordings for Notify") {
		t.Errorf("without --live: skipped %q, failure %q", r.Skipped, r.Failure)
	}
}

func TestLiveContractCallsTheServiceAndRecords(t *testing.T) {
	unsetenv(t, "EYE_SECRETS")
	requests := notifyService(t)
	prog := loadFiles(t, liveNotify)
	dir := prog.Root.Dir
	path := filepath.Join(dir, "recordings", "notify.json")
	// A recording another test replays stays, and the one this test
	// recorded is replaced.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	old := `{"contract": "Notify", "exchanges": [
  {"op": "who", "result": "ann"},
  {"op": "send", "key": "hello-morning", "result": {"kind": "Refused", "error": "old"}}]}`
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}

	results, written := liveRun(t, prog, true, "", "notify")
	if r := results[0]; r.Failure != "" || r.Skipped != "" {
		t.Fatalf("live: failure %q, skipped %q", r.Failure, r.Skipped)
	}
	if r := results[1]; !strings.Contains(r.Skipped, "--live notify: notify.who has no @http, so it can't be called live") {
		t.Errorf("the query without @http: failure %q, skipped %q", r.Failure, r.Skipped)
	}
	if len(*requests) != 1 || (*requests)[0].Header.Get("X-Key") != "hello-morning" {
		t.Fatalf("Notify got %d requests", len(*requests))
	}
	if len(written) != 1 || written[0].Path != path || written[0].Exchanges != 1 {
		t.Fatalf("written %v", written)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := `{
  "contract": "Notify",
  "exchanges": [
    {
      "op": "who",
      "result": "ann"
    },
    {
      "op": "send",
      "key": "hello-morning",
      "result": {
        "deliveries": 2,
        "id": "m-1 for <secret notify-token>",
        "kind": "Accepted"
      }
    }
  ]
}
`
	if string(data) != want {
		t.Errorf("the recording is\n%s\nwant\n%s", data, want)
	}
	if strings.Contains(string(data), "notify-secret-77") {
		t.Error("the recording holds the secret")
	}

	// The next plain run replays it, without calling Notify.
	if r := RunTests(prog, "a send")[0]; r.Failure != "" {
		t.Errorf("replayed: %s", r.Failure)
	}
	if len(*requests) != 1 {
		t.Errorf("the replay called Notify")
	}

	// Recording the same answer again leaves the file as it was.
	liveRun(t, prog, true, "a send", "notify")
	if again, _ := os.ReadFile(path); string(again) != want {
		t.Errorf("recorded again:\n%s", again)
	}
}

func TestFailedLiveTestRecordsNothing(t *testing.T) {
	unsetenv(t, "EYE_SECRETS")
	notifyService(t)
	files := map[string]string{"notify/notify.eye": liveNotify["notify/notify.eye"], "main.eye": `
import notify

job hello(title: String) {
  notify.send!(title, key: "hello-{title}")
}

test "a send, and a wrong expectation" {
  let sent = fakes(notify, from: "recordings/")
  with sent
  hello.start("morning")
  expect(sent.calls(notify.send) == [])
}
`}
	prog := loadFiles(t, files)
	results, written := liveRun(t, prog, true, "", "notify")
	if results[0].Failure == "" || len(written) != 0 {
		t.Errorf("failure %q, written %v", results[0].Failure, written)
	}
	if _, err := os.Stat(filepath.Join(prog.Root.Dir, "recordings", "notify.json")); err == nil {
		t.Error("a failed test wrote a recording")
	}
}

const liveModelTest = askForAnswer + `
entity Answers(name: String) {
  state got: List(String) = []
  pub fn add(text: String) { got += [text] }
}

job keep() {
  let answer = ask.start!().join!()
  Answers("all").add!("{answer}")
}

test "a model counts the boxes" {
  with fakes(ai, from: "recordings/")
  keep.start()
  expect(Answers("all").got == ["Completed(Ok(\{ n: 3, unit: Boxes \}))"])
}
`

func TestLiveModelSkipsWithoutAProvider(t *testing.T) {
	unsetenv(t, "AI_URL", "AI_KEY", "ANTHROPIC_API_KEY", "OPENAI_API_KEY", "EYE_SECRETS")
	prog := loadFiles(t, map[string]string{"main.eye": liveModelTest})
	results, _ := liveRun(t, prog, false, "", "ai")
	if r := results[0]; r.Failure != "" || r.Skipped != "--live ai: no model provider is set: set AI_URL, ANTHROPIC_API_KEY or OPENAI_API_KEY" {
		t.Errorf("skipped %q, failure %q", r.Skipped, r.Failure)
	}

	// A provider is set, but not the one the model's name picks.
	t.Setenv("OPENAI_API_KEY", "openai-key")
	claude := strings.Replace(liveModelTest, `model: "m"`, `model: "claude-x"`, 1)
	results, _ = liveRun(t, loadFiles(t, map[string]string{"main.eye": claude}), false, "", "ai")
	if r := results[0]; r.Failure != "" || r.Skipped != "--live ai: ai.generate asks Anthropic for claude-x, and ANTHROPIC_API_KEY isn't set" {
		t.Errorf("skipped %q, failure %q", r.Skipped, r.Failure)
	}
}

func TestLiveModelAnswersAndIsRecorded(t *testing.T) {
	unsetenv(t, "ANTHROPIC_API_KEY", "OPENAI_API_KEY", "EYE_SECRETS")
	t.Setenv("AI_KEY", "gateway-secret-5")
	t.Setenv("AI_PRICES", "m=1/4")
	var auth []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = append(auth, r.Header.Get("Authorization"))
		// The reply has a field the schema doesn't ask for, which repeats
		// the key.
		out, _ := json.Marshal(map[string]any{
			"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{
				"content": `{"n": 3, "unit": "Boxes", "seen": "gateway-secret-5"}`}}},
			"usage": map[string]any{"prompt_tokens": 1000, "completion_tokens": 50},
		})
		_, _ = w.Write(out)
	}))
	defer srv.Close()
	t.Setenv("AI_URL", srv.URL)
	prog := loadFiles(t, map[string]string{"main.eye": liveModelTest})

	results, written := liveRun(t, prog, true, "", "ai")
	if r := results[0]; r.Failure != "" || r.Skipped != "" {
		t.Fatalf("live: failure %q, skipped %q", r.Failure, r.Skipped)
	}
	if len(auth) != 1 || auth[0] != "Bearer gateway-secret-5" {
		t.Fatalf("the gateway was sent %v", auth)
	}
	path := filepath.Join(prog.Root.Dir, "recordings", "ai.json")
	if len(written) != 1 || written[0].Path != path {
		t.Fatalf("written %v", written)
	}
	data, _ := os.ReadFile(path)
	want := `{
  "contract": "ai",
  "exchanges": [
    {
      "result": {
        "n": 3,
        "seen": "<secret ai-key>",
        "unit": "Boxes"
      },
      "usage": {
        "input": 1000,
        "output": 50,
        "usd": 0.0012
      }
    }
  ]
}
`
	if string(data) != want {
		t.Errorf("the recording is\n%s\nwant\n%s", data, want)
	}

	// The next plain run replays it, with the gateway gone.
	srv.Close()
	unsetenv(t, "AI_URL", "AI_KEY")
	if r := RunTests(prog, "")[0]; r.Failure != "" {
		t.Errorf("replayed: %s", r.Failure)
	}
}

func TestLiveModulesAreNamedRight(t *testing.T) {
	prog := loadFiles(t, map[string]string{
		"notify/notify.eye": liveNotify["notify/notify.eye"],
		"words/words.eye":   "pub fn twice(s: String) -> String = s + s\n",
		"main.eye":          liveModelTest + "\nimport notify\nimport words\n\nlet doubled = words.twice(\"a\")\n",
	})
	for name, want := range map[string]string{
		"nosuch": "--live nosuch: the program has no module nosuch",
		"words":  "--live words: words declares 0 contracts, and --live takes modules that declare one, as fakes(…) does",
	} {
		_, err := LiveModules(prog, []string{name})
		if err == nil || err.Error() != want {
			t.Errorf("--live %s: %v, want %s", name, err, want)
		}
	}
	mods, err := LiveModules(prog, []string{"ai", "notify"})
	if err != nil || len(mods) != 2 {
		t.Errorf("--live ai,notify: %v, %v", mods, err)
	}
}

func TestLiveHTTPIsRecordedByURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "hello")
	}))
	defer srv.Close()
	src := `import http

fn fetch(url: String) -> String uses {net} =
  match http.get(url) {
    Ok(r) => "{r.status}: {r.body}"
    Err(e) => "{e}"
  }

test "a page" {
  with fakes(http, from: "rec/")
  expect(fetch("URL/page") == "200: hello")
}
`
	prog := loadFiles(t, map[string]string{"main.eye": strings.Replace(src, "URL", srv.URL, 1)})
	results, written := liveRun(t, prog, true, "", "http")
	if r := results[0]; r.Failure != "" || r.Skipped != "" {
		t.Fatalf("live: failure %q, skipped %q", r.Failure, r.Skipped)
	}
	path := filepath.Join(prog.Root.Dir, "rec", "http.json")
	if len(written) != 1 || written[0].Path != path {
		t.Fatalf("written %v", written)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `"key": "`+srv.URL+`/page"`) {
		t.Errorf("the recording is\n%s", data)
	}

	// With the server gone, a plain run replays the page.
	srv.Close()
	if r := RunTests(prog, "")[0]; r.Failure != "" {
		t.Errorf("replayed: %s", r.Failure)
	}
}
