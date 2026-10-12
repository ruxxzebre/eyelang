package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A program with a test that sends through a contract service and a test
// that asks a model, both through fakes that replay recordings it doesn't
// have yet.
var liveProgram = map[string]string{
	"notify/notify.eye": `pub type Delivery =
  | Accepted(id: String)
  | Refused(error: String)

@auth(bearer: "notify-token")
contract Notify {
  @http("POST /v1/send", status: { 200: Accepted, 400: Refused })
  @key(required, header: "X-Key")
  command send(title: String) -> Delivery
}
`,
	"main.eye": `import ai
import notify

type Answer = { n: Int, unit: Unit }

type Unit = Items | Boxes

entity Answers(name: String) {
  state got: List(String) = []
  pub fn add(text: String) { got += [text] }
}

job hello(title: String) {
  notify.send!(title, key: "hello-{title}")
}

job count() {
  let answer: Result(Answer, ai.AiError) = ai.generate!(model: "m", system: "Count.", prompt: "three boxes")
  Answers("all").add!("{answer}")
}

test "a send goes to Notify" {
  let sent = fakes(notify, from: "recordings/")
  with sent
  hello.start("morning")
  expect(sent.calls(notify.send) == [{ title: "morning", key: "hello-morning" }])
}

test "a model counts the boxes" {
  with fakes(ai, from: "recordings/")
  count.start()
  expect(Answers("all").got == ["Ok(\{ n: 3, unit: Boxes \})"])
}
`,
}

// The task's test (ROADMAP.md 6.9, #61): eye test --live skips the tests
// whose services aren't set, calls the ones that are, --record writes what
// they answered without the secrets, and a plain eye test replays it.
func TestTestLive(t *testing.T) {
	for _, n := range []string{"NOTIFY_URL", "NOTIFY_TOKEN", "AI_URL", "AI_KEY", "ANTHROPIC_API_KEY", "OPENAI_API_KEY", "EYE_SECRETS"} {
		t.Setenv(n, "")
		os.Unsetenv(n)
	}
	dir := program(t, liveProgram)
	var stdout, stderr bytes.Buffer
	eyeTest := func(args ...string) int {
		stdout.Reset()
		stderr.Reset()
		return run(append(append([]string{"test"}, args...), dir), &stdout, &stderr)
	}

	// Nothing is set: both tests are skipped, and that isn't a failure.
	if code := eyeTest("--live", "ai,notify"); code != 0 {
		t.Fatalf("eye test --live with nothing set: exit %d\n%s%s", code, stdout.String(), stderr.String())
	}
	for _, want := range []string{
		"skip  a send goes to Notify\n      --live notify: set NOTIFY_URL, the address of its service, and NOTIFY_TOKEN, its secret \"notify-token\"\n",
		"skip  a model counts the boxes\n      --live ai: no model provider is set: set AI_URL, ANTHROPIC_API_KEY or OPENAI_API_KEY\n",
		"0 tests passed, 2 skipped",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("eye test --live printed\n%s\nwant %q", stdout.String(), want)
		}
	}

	// Stand-ins for Notify and the model gateway, which repeat the secret
	// they were sent in what they answer.
	var notified, asked int
	notify := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		notified++
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		_, _ = io.WriteString(w, `{"id": "m-1 for `+token+`"}`)
	}))
	defer notify.Close()
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked++
		key := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		out, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop",
			"message": map[string]any{"content": `{"n": 3, "unit": "Boxes", "seen": "` + key + `"}`}}}})
		_, _ = w.Write(out)
	}))
	defer gateway.Close()
	t.Setenv("NOTIFY_URL", notify.URL)
	t.Setenv("NOTIFY_TOKEN", "notify-secret-77")
	t.Setenv("AI_URL", gateway.URL)
	t.Setenv("AI_KEY", "gateway-secret-5")

	// --live alone calls them and writes nothing.
	if code := eyeTest("--live", "ai,notify"); code != 0 || notified != 1 || asked != 1 {
		t.Fatalf("eye test --live: exit %d, %d sends, %d model calls\n%s%s", code, notified, asked, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "recordings")); err == nil {
		t.Error("eye test --live without --record wrote recordings")
	}

	if code := eyeTest("--live", "ai,notify", "--record"); code != 0 {
		t.Fatalf("eye test --live --record: exit %d\n%s%s", code, stdout.String(), stderr.String())
	}
	notifyFile := filepath.Join(dir, "recordings", "notify.json")
	aiFile := filepath.Join(dir, "recordings", "ai.json")
	for _, want := range []string{
		"ok    a send goes to Notify",
		"ok    a model counts the boxes",
		"recorded " + notifyFile + " (1 exchange)",
		"recorded " + aiFile + " (1 exchange)",
		"2 tests passed",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("eye test --live --record printed\n%s\nwant %q", stdout.String(), want)
		}
	}
	for _, path := range []string{notifyFile, aiFile} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, secret := range []string{"notify-secret-77", "gateway-secret-5"} {
			if strings.Contains(string(data), secret) {
				t.Errorf("%s holds the secret %s:\n%s", path, secret, data)
			}
		}
	}

	// With the services gone, a plain eye test replays what was recorded.
	notify.Close()
	gateway.Close()
	for _, n := range []string{"NOTIFY_URL", "NOTIFY_TOKEN", "AI_URL", "AI_KEY"} {
		os.Unsetenv(n)
	}
	if code := eyeTest(); code != 0 || !strings.Contains(stdout.String(), "2 tests passed") {
		t.Errorf("eye test after recording: exit %d\n%s%s", code, stdout.String(), stderr.String())
	}

	// --record needs --live.
	if code := eyeTest("--record"); code != exitUsage || !strings.Contains(stderr.String(), "--record writes what the real services answered, so it needs --live") {
		t.Errorf("eye test --record alone: exit %d, %q", code, stderr.String())
	}
	if code := eyeTest("--live", "nosuch"); code != exitUsage || !strings.Contains(stderr.String(), "--live nosuch: the program has no module nosuch") {
		t.Errorf("eye test --live nosuch: exit %d, %q", code, stderr.String())
	}
}
