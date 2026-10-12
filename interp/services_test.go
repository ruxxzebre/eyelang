package interp

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"eyelang/check"
	"eyelang/ir"
)

// TestDigestOverHTTP runs the lab's morning digest against stand-ins for
// the services it calls: Miniflux, the VPN's proxy, FlareSolverr, the model
// gateway and Notify. The first send to Notify loses its reply, so the
// digest reconciles it by sending again with the same key.
func TestDigestOverHTTP(t *testing.T) {
	var mu sync.Mutex
	var sends []*http.Request
	var sendBodies []map[string]any
	var modelRequest map[string]any
	var flareRequest map[string]any

	miniflux := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Auth-Token") != "mf-key" {
			http.Error(w, `{"error_message":"bad key"}`, http.StatusUnauthorized)
			return
		}
		switch {
		case r.URL.Path == "/v1/me":
			_, _ = io.WriteString(w, `{"id": 1, "username": "digest"}`)
		case r.URL.Path == "/v1/categories":
			var list []map[string]any
			for id, title := range productionCategories() {
				list = append(list, map[string]any{"id": id, "title": title})
			}
			_ = json.NewEncoder(w).Encode(list)
		case r.URL.Path == "/v1/feeds" && r.Method == "GET":
			_, _ = io.WriteString(w, `[]`)
		case r.URL.Path == "/v1/feeds":
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"feed_id": 1}`)
		case strings.HasPrefix(r.URL.Path, "/v1/categories/") && strings.HasSuffix(r.URL.Path, "/entries"):
			q := r.URL.Query()
			if q.Get("limit") != "60" || q.Get("published_after") == "" || len(q["status"]) != 2 || q.Get("order") != "published_at" {
				t.Errorf("entries query = %s", r.URL.RawQuery)
			}
			_, _ = io.WriteString(w, `{"total": 1, "entries": [{"id": 9, "title": "Rent cap extended",
				"url": "https://rte.ie/rent", "published_at": "2026-10-12T05:00:00Z",
				"content": "<p>The rent cap is extended to 2027.</p>", "feed": {"title": "RTÉ"}}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer miniflux.Close()

	// The VPN's proxy, which FlareSolverr is told to use. Pages read with
	// http.get get offline's page instead, so the test reads no website.
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no route", http.StatusBadGateway)
	}))
	defer proxy.Close()

	flare := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		_ = json.NewDecoder(r.Body).Decode(&flareRequest)
		mu.Unlock()
		_, _ = io.WriteString(w, `{"status": "ok", "message": "", "solution": {"url": "https://www.dublincity.ie/events",
			"status": 200, "response": "<html>Culture Night, Friday</html>"}}`)
	}))
	defer flare.Close()

	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer ai-secret" {
			http.Error(w, "wrong path or key", 400)
			return
		}
		mu.Lock()
		_ = json.NewDecoder(r.Body).Decode(&modelRequest)
		mu.Unlock()
		briefing := `{"highlights": ["Rent cap extended to 2027"], "sections": [
			{"heading": "Temporary protection", "stories": []},
			{"heading": "Dublin", "stories": [{"headline": "Rent cap extended", "text": "It now runs to 2027.",
				"action": null, "sources": [{"title": "RTÉ", "url": "https://rte.ie/rent"}]}]},
			{"heading": "Coming up in Dublin", "stories": []}]}`
		reply, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{
			"finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": briefing}}},
			"usage": map[string]any{"prompt_tokens": 2000, "completion_tokens": 400}})
		_, _ = w.Write(reply)
	}))
	defer gateway.Close()

	notify := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		sends = append(sends, r)
		sendBodies = append(sendBodies, body)
		first := len(sends) == 1
		mu.Unlock()
		if first {
			// The request arrived, and the reply is lost.
			conn, _, _ := w.(http.Hijacker).Hijack()
			conn.Close()
			return
		}
		_, _ = io.WriteString(w, `{"id": "msg-1", "deliveries": 1, "deduped": true}`)
	}))
	defer notify.Close()

	t.Setenv("MINIFLUX_URL", miniflux.URL)
	t.Setenv("MINIFLUX_KEY", "mf-key")
	t.Setenv("VPN_PROXY_URL", proxy.URL)
	t.Setenv("FLARESOLVERR_URL", flare.URL)
	t.Setenv("AI_URL", gateway.URL+"/v1")
	t.Setenv("AI_KEY", "ai-secret")
	t.Setenv("DIGEST_MODEL", "test-model")
	t.Setenv("AI_PRICES", "test-model=1/4") // the digest's @budget is in dollars
	t.Setenv("NOTIFY_URL", notify.URL)
	t.Setenv("NOTIFY_TOKEN", "notify-secret")

	prog, diags := check.Program("../testdata/lab")
	for _, d := range diags {
		t.Fatal(d)
	}
	var log bytes.Buffer
	in := New(prog, &log, &log)
	offline(in) // the pages read directly
	dublin, _ := time.LoadLocation("Europe/Dublin")
	in.rt.now = time.Date(2026, 10, 12, 7, 40, 0, 0, dublin)
	digest := moduleByPath(t, prog, "digest")
	day := ir.Date{Year: 2026, Month: 10, Day: 12}
	run := in.rt.startNow(digest.Jobs["morning"], []Value{day}, nil, ir.Pos{})
	for _, f := range in.rt.failures {
		t.Fatalf("%v\n%s", f, log.String())
	}

	data, _ := in.rt.store.Run(run.(*RunVal).ID)
	if r := decodeRun(data); r.Status != "done" {
		t.Fatalf("morning is %s (%s)\n%s", r.Status, r.Fault, log.String())
	} else if got := show(in.decode(r.Result, ir.Pos{})); !strings.HasPrefix(got, "Ok(Accepted(") {
		t.Errorf("morning returned %s", got)
	}

	// The model was asked for a Briefing, by its schema.
	format, _ := modelRequest["response_format"].(map[string]any)
	schema, _ := format["json_schema"].(map[string]any)["schema"].(map[string]any)
	if modelRequest["model"] != "test-model" || schema["type"] != "object" ||
		schema["description"] != "A morning briefing for one reader." {
		t.Errorf("model request = %v", modelRequest)
	}
	if props, _ := schema["properties"].(map[string]any); props["highlights"] == nil || props["sections"] == nil {
		t.Errorf("schema = %v", schema)
	}
	// Each write's model call recorded what it spent: 2,400 tokens at $1 in
	// and $4 out per million.
	calls := 0
	for _, id := range in.rt.store.Runs() {
		data, _ := in.rt.store.Run(id)
		for _, s := range decodeRun(data).history() {
			if s.Call != "ai.generate" {
				continue
			}
			calls++
			if s.Tokens != 2400 || math.Abs(s.USD-0.0036) > 1e-9 {
				t.Errorf("the model call's step spent %d tokens and $%g", s.Tokens, s.USD)
			}
		}
	}
	if calls == 0 {
		t.Error("no step called ai.generate")
	}
	if via, _ := flareRequest["proxy"].(map[string]any); flareRequest["cmd"] != "request.get" || via["url"] != proxy.URL {
		t.Errorf("FlareSolverr request = %v", flareRequest)
	}

	// Notify got the same send twice: once lost, once answered.
	if len(sends) != 2 {
		t.Fatalf("Notify got %d sends, want 2", len(sends))
	}
	for i, r := range sends {
		if r.Header.Get("X-Actions-Idempotency-Key") != "ireland-2026-10-12" || r.Header.Get("Authorization") != "Bearer notify-secret" {
			t.Errorf("send %d headers = %v", i+1, r.Header)
		}
		b := sendBodies[i]
		if b["event"] != "digest.ireland" || b["priority"] != "normal" || b["format"] != "markdown" ||
			b["dedupe_key"] != "ireland-2026-10-12" || b["title"] != "Ireland & Dublin — Monday 12 October" {
			t.Errorf("send %d body = %v", i+1, b)
		}
		if _, ok := b["to"]; ok {
			t.Errorf("send %d has a to, which Notify refuses when empty: %v", i+1, b["to"])
		}
	}
	text, _ := sendBodies[1]["body"].(string)
	for _, want := range []string{"- Rent cap extended to 2027", "## Dublin", "**Rent cap extended**", "[RTÉ](https://rte.ie/rent)", "Embassy of Ukraine in Ireland"} {
		if !strings.Contains(text, want) {
			t.Errorf("the briefing has no %q:\n%s", want, text)
		}
	}
}

// loadFiles checks a program made of the given files, by path.
func loadFiles(t *testing.T, files map[string]string) *ir.Program {
	t.Helper()
	dir := t.TempDir()
	for name, src := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	prog, diags := check.Program(dir)
	for _, d := range diags {
		t.Fatal(d)
	}
	return prog
}

// runJob starts a job of the program's own module and returns its outcome.
func runJob(t *testing.T, in *Interp, name string, args ...Value) string {
	t.Helper()
	run := in.rt.startNow(in.prog.Root.Jobs[name], args, nil, ir.Pos{})
	data, _ := in.rt.store.Run(run.(*RunVal).ID)
	return show(in.outcome(decodeRun(data), ir.Pos{}))
}

const askForAnswer = `
import ai

## How many there are.
type Answer = { n: Int, unit: Unit }

type Unit = Items | Boxes

job ask() -> Result(Answer, ai.AiError) {
  ai.generate!(model: "m", system: "Count.", prompt: "three boxes")
}
`

// gateway answers chat completions with the replies given, in order, and
// keeps the requests.
func gateway(t *testing.T, replies ...string) (*httptest.Server, *[]map[string]any) {
	var mu sync.Mutex
	var requests []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		reply := replies[min(len(requests), len(replies)-1)]
		requests = append(requests, body)
		mu.Unlock()
		out, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{
			"finish_reason": "stop", "message": map[string]any{"content": reply}}}})
		_, _ = w.Write(out)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("AI_URL", srv.URL)
	return srv, &requests
}

func TestModelReplyIsCheckedAndAskedAgain(t *testing.T) {
	_, requests := gateway(t, `{"n": "three"}`, `{"n": 3, "unit": "Boxes"}`)
	in := New(load(t, askForAnswer), io.Discard, io.Discard)
	if got := runJob(t, in, "ask"); got != "Completed(Ok({ n: 3, unit: Boxes }))" {
		t.Fatalf("ask = %s", got)
	}
	if len(*requests) != 2 {
		t.Fatalf("%d requests, want 2", len(*requests))
	}
	schema := (*requests)[0]["response_format"].(map[string]any)["json_schema"].(map[string]any)["schema"].(map[string]any)
	unit := schema["properties"].(map[string]any)["unit"].(map[string]any)
	if schema["description"] != "How many there are." || len(unit["enum"].([]any)) != 2 {
		t.Errorf("schema = %v", schema)
	}
	again := (*requests)[1]["messages"].([]any)
	if last := again[len(again)-1].(map[string]any)["content"].(string); !strings.Contains(last, "n: expected a whole number") {
		t.Errorf("the second request says %q", last)
	}
}

func TestMalformedReplies(t *testing.T) {
	gateway(t, `{"n": "three"}`)
	in := New(load(t, askForAnswer), io.Discard, io.Discard)
	got := runJob(t, in, "ask")
	if !strings.HasPrefix(got, `Completed(Err(Malformed(reply: "{\"n\": \"three\"}"`) {
		t.Errorf("ask = %s", got)
	}
}

func TestModelRepliesReplayInTests(t *testing.T) {
	prog := loadFiles(t, map[string]string{
		"main.eye": askForAnswer + `
test "a recorded reply" {
  with fakes(ai, from: "recordings/")
  let run = ask.start()
}
`,
		"recordings/ai.json": `{"contract": "ai", "exchanges": [{"op": "generate", "result": {"n": 3, "unit": "Items"}}]}`,
	})
	for _, r := range RunTests(prog, "") {
		if r.Failure != "" {
			t.Errorf("%s: %s", r.Name, r.Failure)
		}
	}
}

func TestUnreachableQueryFailsTheRun(t *testing.T) {
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "busy", http.StatusServiceUnavailable)
	}))
	defer down.Close()
	t.Setenv("STORE_URL", down.URL)
	in := New(loadFiles(t, map[string]string{
		"store/store.eye": `
pub type Item = { name: String }
pub type Saved = | Saved

contract Store {
  @http("GET /items/:name")
  query item(name: String) -> Item

  @http("POST /items", status: {201: Saved})
  command save(name: String) -> Saved
}
`,
		"main.eye": `
import store

job fetch() -> store.Item { store.item!("a") }

job outer() -> String {
  match fetch.start!().join!() {
    Completed(item) => item.name
    Failed(fault) => "failed: {fault}"
  }
}

job put() -> store.Saved {
  match store.save!("a") {
    Saved => Saved
    Uncertain(_) => unreachable("never sent, so not uncertain")
  }
}
`,
	}), io.Discard, io.Discard)
	in.rt.fastRetries = true
	if got := runJob(t, in, "outer"); got != `Completed("failed: Unavailable(\"store.item\")")` {
		t.Errorf("outer = %s", got)
	}
	// A command that never reached the service is retried, not uncertain.
	down.Close()
	if got := runJob(t, in, "put"); got != `Failed(Unavailable("store.save"))` {
		t.Errorf("put = %s", got)
	}
}

func moduleByPath(t *testing.T, prog *ir.Program, path string) *ir.Module {
	t.Helper()
	for _, m := range prog.Modules {
		if m.Path == path {
			return m
		}
	}
	t.Fatalf("no module %s", path)
	return nil
}

// TestFreshAndDestructiveOverHTTP checks the JSON shape of SPECS.md §17 for
// @fresh and @destructive commands: basedOn and approval go as arguments, a
// 409 is Stale and a 410 Expired, and a body can say either too.
func TestFreshAndDestructiveOverHTTP(t *testing.T) {
	var mu sync.Mutex
	var bodies []map[string]any
	clears := []struct {
		status int
		body   string
	}{
		{http.StatusConflict, `{"error": "changed"}`},
		{http.StatusGone, ``},
		{http.StatusOK, `{"kind": "Stale"}`},
		{http.StatusOK, `"expired"`},
		{http.StatusOK, `{"kind": "Cleared", "count": 2}`},
		{http.StatusPreconditionFailed, ``},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/shelves/a":
			_, _ = io.WriteString(w, `{"id": "a", "version": "W/\"41\""}`)
		case r.Method == "POST" && r.URL.Path == "/shelves/a/clear":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			mu.Lock()
			bodies = append(bodies, body)
			reply := clears[len(bodies)-1]
			mu.Unlock()
			w.WriteHeader(reply.status)
			_, _ = io.WriteString(w, reply.body)
		case r.Method == "POST" && r.URL.Path == "/shelves/a/burn":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			mu.Lock()
			bodies = append(bodies, body)
			mu.Unlock()
			_, _ = io.WriteString(w, `"burned"`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	t.Setenv("SHELVES_URL", srv.URL)
	in := New(loadFiles(t, map[string]string{
		"shelves/shelves.eye": `
pub type Shelf = { id: String, version: Version }
pub type Cleared = | Cleared(count: Int)
pub type Burned = | Burned

@http(names: snake)
contract Shelves {
  @http("GET /shelves/:id")
  query shelf(id: String) -> Shelf

  @fresh
  @http("POST /shelves/:id/clear", status: { 412: Stale })
  command clear(id: String) -> Cleared

  @destructive
  @http("POST /shelves/:id/burn")
  command burn(id: String) -> Burned
}
`,
		"main.eye": `
import shelves.{Cleared, Burned}

job tidy(id: String) -> String {
  let shelf = shelves.shelf!(id)
  match shelves.clear!(id, basedOn: shelf.version) {
    Cleared(n) => "cleared {n}"
    Stale => "stale"
    Expired => "expired"
    Uncertain(_) => "uncertain"
  }
}

job burnIt(id: String, ok: Approval) -> String {
  match shelves.burn!(id, approval: ok) {
    Burned => "burned"
    Uncertain(_) => "uncertain"
  }
}
`,
	}), io.Discard, io.Discard)
	for i, want := range []string{"stale", "expired", "stale", "expired", "cleared 2", "stale"} {
		if got := runJob(t, in, "tidy", "a"); got != `Completed("`+want+`")` {
			t.Errorf("clear %d = %s, want %s", i+1, got, want)
		}
	}
	if got := runJob(t, in, "burnIt", "a", &ApprovalVal{What: "Burn shelf a?", By: "ann"}); got != `Completed("burned")` {
		t.Errorf("burn = %s", got)
	}
	mu.Lock()
	defer mu.Unlock()
	for i, body := range bodies[:len(clears)] {
		if body["based_on"] != `W/"41"` || len(body) != 1 {
			t.Errorf("clear %d sent %v, want based_on as the shelf's version", i+1, body)
		}
	}
	approval, _ := bodies[len(clears)]["approval"].(map[string]any)
	if approval["what"] != "Burn shelf a?" || approval["by"] != "ann" {
		t.Errorf("burn sent %v, want the approval's what and by", bodies[len(clears)])
	}
}
