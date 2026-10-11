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
	"time"

	"eyelang/ir"
)

// stand-in is a model provider in a test: it answers each request with the
// next reply, the last one again when they run out, and keeps the requests.
type standIn struct {
	mu       sync.Mutex
	replies  []map[string]any
	requests []map[string]any
	headers  []http.Header
	paths    []string
}

func (s *standIn) serve(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		s.mu.Lock()
		reply := s.replies[min(len(s.requests), len(s.replies)-1)]
		s.requests = append(s.requests, body)
		s.headers = append(s.headers, r.Header.Clone())
		s.paths = append(s.paths, r.URL.Path)
		s.mu.Unlock()
		out, _ := json.Marshal(reply)
		_, _ = w.Write(out)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// anthropicReply is a Messages API answer.
func anthropicReply(text, stop string, in, out int64) map[string]any {
	return map[string]any{
		"type": "message", "role": "assistant", "stop_reason": stop,
		"content": []any{map[string]any{"type": "text", "text": text}},
		"usage":   map[string]any{"input_tokens": in, "output_tokens": out},
	}
}

// chatReply is a chat completions answer.
func chatReply(text, finish string, in, out int64) map[string]any {
	return map[string]any{
		"choices": []any{map[string]any{"finish_reason": finish, "message": map[string]any{"content": text}}},
		"usage":   map[string]any{"prompt_tokens": in, "completion_tokens": out},
	}
}

// noGateway makes the model's name pick the provider.
func noGateway(t *testing.T) {
	t.Setenv("AI_URL", "")
	t.Setenv("AI_PRICES", "")
}

const askClaude = `
import ai

## How many there are.
type Answer = { n: Int, unit: Unit }

type Unit = Items | Boxes

job ask() -> Result(Answer, ai.AiError) {
  ai.generate!(model: "claude-sonnet-5-5", system: "Count.", prompt: "three boxes")
}
`

func TestAnthropicAnswersClaude(t *testing.T) {
	noGateway(t)
	api := &standIn{replies: []map[string]any{
		anthropicReply(`{"n": "three"}`, "end_turn", 300, 20),
		anthropicReply(`{"n": 3, "unit": "Boxes"}`, "end_turn", 340, 20),
	}}
	t.Setenv("ANTHROPIC_URL", api.serve(t).URL)
	t.Setenv("ANTHROPIC_API_KEY", "claude-key")
	in := New(load(t, askClaude), io.Discard, io.Discard)
	if got := runJob(t, in, "ask"); got != "Completed(Ok({ n: 3, unit: Boxes }))" {
		t.Fatalf("ask = %s", got)
	}
	if len(api.requests) != 2 {
		t.Fatalf("%d requests, want 2: the first reply didn't match", len(api.requests))
	}
	first := api.requests[0]
	if api.paths[0] != "/v1/messages" || api.headers[0].Get("x-api-key") != "claude-key" ||
		api.headers[0].Get("anthropic-version") != anthropicVersion {
		t.Errorf("request went to %s with headers %v", api.paths[0], api.headers[0])
	}
	if first["model"] != "claude-sonnet-5-5" || first["system"] != "Count." || first["max_tokens"] != float64(defaultMaxReply) {
		t.Errorf("request = %v", first)
	}
	format := first["output_config"].(map[string]any)["format"].(map[string]any)
	if format["type"] != "json_schema" || format["schema"].(map[string]any)["description"] != "How many there are." {
		t.Errorf("output_config.format = %v", format)
	}
	// The second request carries the first reply and what was wrong with it.
	again := api.requests[1]["messages"].([]any)
	if len(again) != 3 || again[1].(map[string]any)["role"] != "assistant" ||
		!strings.Contains(again[2].(map[string]any)["content"].(string), "n: expected a whole number") {
		t.Errorf("the second request's messages = %v", again)
	}
}

func TestAnthropicRefusalIsDeclined(t *testing.T) {
	noGateway(t)
	refusal := anthropicReply("", "refusal", 300, 0)
	refusal["stop_details"] = map[string]any{"type": "refusal", "category": "cyber", "explanation": "not this one"}
	api := &standIn{replies: []map[string]any{refusal}}
	t.Setenv("ANTHROPIC_URL", api.serve(t).URL)
	t.Setenv("ANTHROPIC_API_KEY", "claude-key")
	in := New(load(t, askClaude), io.Discard, io.Discard)
	if got := runJob(t, in, "ask"); got != `Completed(Err(Declined(reason: "not this one")))` {
		t.Errorf("ask = %s", got)
	}
}

func TestProviderFollowsTheModelOrProviderArgument(t *testing.T) {
	noGateway(t)
	openai := &standIn{replies: []map[string]any{chatReply(`{"n": 1, "unit": "Items"}`, "stop", 10, 5)}}
	gateway := &standIn{replies: []map[string]any{chatReply(`{"n": 2, "unit": "Items"}`, "stop", 10, 5)}}
	anthropic := &standIn{replies: []map[string]any{anthropicReply(`{"n": 3, "unit": "Items"}`, "end_turn", 10, 5)}}
	t.Setenv("OPENAI_URL", openai.serve(t).URL+"/v1")
	t.Setenv("OPENAI_API_KEY", "openai-key")
	t.Setenv("ANTHROPIC_URL", anthropic.serve(t).URL)
	t.Setenv("ANTHROPIC_API_KEY", "claude-key")
	gatewayURL := gateway.serve(t).URL + "/v1"
	prog := load(t, `
import ai

type Answer = { n: Int, unit: Unit }
type Unit = Items | Boxes

job byName(model: String) -> Result(Answer, ai.AiError) {
  ai.generate!(model:, prompt: "how many")
}

job viaGateway() -> Result(Answer, ai.AiError) {
  ai.generate!(model: "claude-opus-5-5", prompt: "how many", provider: Some(ai.Gateway))
}

job viaAnthropic() -> Result(Answer, ai.AiError) {
  ai.generate!(model: "claude-opus-5-5", prompt: "how many", provider: Some(ai.Anthropic))
}
`)
	in := New(prog, io.Discard, io.Discard)
	if got := runJob(t, in, "byName", "gpt-5.5"); got != "Completed(Ok({ n: 1, unit: Items }))" {
		t.Errorf("gpt-5.5 = %s", got)
	}
	if openai.paths[0] != "/v1/chat/completions" || openai.headers[0].Get("Authorization") != "Bearer openai-key" {
		t.Errorf("OpenAI got %s with %v", openai.paths[0], openai.headers[0])
	}
	if _, ok := openai.requests[0]["max_completion_tokens"]; ok {
		t.Errorf("without a budget, the request has no limit: %v", openai.requests[0])
	}
	if got := runJob(t, in, "byName", "claude-haiku-5-5"); got != "Completed(Ok({ n: 3, unit: Items }))" {
		t.Errorf("claude-haiku-5-5 = %s", got)
	}

	// A gateway, once there is one, answers every model.
	t.Setenv("AI_URL", gatewayURL)
	if got := runJob(t, in, "byName", "claude-haiku-5-5"); got != "Completed(Ok({ n: 2, unit: Items }))" {
		t.Errorf("claude-haiku-5-5 with a gateway = %s", got)
	}
	// And provider: says otherwise.
	if got := runJob(t, in, "viaAnthropic"); got != "Completed(Ok({ n: 3, unit: Items }))" {
		t.Errorf("viaAnthropic = %s", got)
	}
	t.Setenv("AI_URL", "")
	if got := runJob(t, in, "viaGateway"); !strings.Contains(got, "set AI_URL") {
		t.Errorf("viaGateway without AI_URL = %s", got)
	}
	if len(anthropic.requests) != 2 || len(gateway.requests) != 1 || len(openai.requests) != 1 {
		t.Errorf("requests: Anthropic %d, gateway %d, OpenAI %d", len(anthropic.requests), len(gateway.requests), len(openai.requests))
	}
}

func TestTokenBudgetStopsTheCallThatWouldGoOver(t *testing.T) {
	noGateway(t)
	api := &standIn{replies: []map[string]any{anthropicReply(`{"n": 3, "unit": "Boxes"}`, "end_turn", 1500, 500)}}
	t.Setenv("ANTHROPIC_URL", api.serve(t).URL)
	t.Setenv("ANTHROPIC_API_KEY", "claude-key")
	prog := load(t, `
import ai

type Answer = { n: Int, unit: Unit }
type Unit = Items | Boxes

@budget(tokens: 2_050)
job twice() -> Result(Answer, ai.AiError) {
  let first: Answer = ai.generate!(model: "claude-sonnet-5-5", prompt: "how many")?
  ai.generate!(model: "claude-sonnet-5-5", prompt: "and now")
}
`)
	in := New(prog, io.Discard, io.Discard)
	if got := runJob(t, in, "twice"); got != "Failed(OverBudget)" {
		t.Fatalf("twice = %s", got)
	}
	// The first call was capped where the budget ends; the second, with 50
	// tokens left, would need more and was never sent.
	if len(api.requests) != 1 {
		t.Fatalf("%d requests, want 1", len(api.requests))
	}
	if limit := api.requests[0]["max_tokens"].(float64); limit >= 2050 || limit < 1900 {
		t.Errorf("the first call's max_tokens = %v, want the budget less its prompt", limit)
	}
	data, _ := in.rt.store.Run(1)
	if run := decodeRun(data); !strings.Contains(run.Fault, "spent 2000 of its 2050 tokens") {
		t.Errorf("fault = %q", run.Fault)
	}
}

func TestDollarBudgetCountsAcrossARestart(t *testing.T) {
	noGateway(t)
	// claude-opus-5-5 costs $4 in and $20 out per million tokens: each call
	// here costs $0.004 + $0.005 = $0.009 of the $0.01.
	api := &standIn{replies: []map[string]any{anthropicReply(`{"n": 3, "unit": "Boxes"}`, "end_turn", 1000, 250)}}
	t.Setenv("ANTHROPIC_URL", api.serve(t).URL)
	t.Setenv("ANTHROPIC_API_KEY", "claude-key")
	prog := load(t, `
import ai

type Answer = { n: Int, unit: Unit }
type Unit = Items | Boxes

@budget(usd: 0.01)
job twice() -> Result(Answer, ai.AiError) {
  let first: Answer = ai.generate!(model: "claude-opus-5-5", prompt: "how many")?
  sleep!(1 h)
  ai.generate!(model: "claude-opus-5-5", prompt: "and now")
}
`)
	in := New(prog, io.Discard, io.Discard)
	in.rt.startNow(prog.Root.Jobs["twice"], nil, nil, ir.Pos{})

	// A new interpreter on the same store, as after a restart, replays the
	// first call's step, and counts what it spent.
	again := New(prog, io.Discard, io.Discard)
	again.rt.useStore(in.rt.index.Store)
	again.rt.now, again.rt.nextRun = in.rt.now, in.rt.nextRun
	again.rt.advance(ir.Duration(time.Hour))
	data, _ := again.rt.store.Run(1)
	run := decodeRun(data)
	if got := show(again.outcome(run, ir.Pos{})); got != "Failed(OverBudget)" {
		t.Fatalf("twice = %s (%s)", got, run.Fault)
	}
	if len(api.requests) != 1 {
		t.Errorf("%d requests, want 1: the second call doesn't fit", len(api.requests))
	}
	if !strings.Contains(run.Fault, "spent $0.0090 of its $0.01") {
		t.Errorf("fault = %q", run.Fault)
	}
}

func TestReplyCutOffByTheBudget(t *testing.T) {
	noGateway(t)
	api := &standIn{replies: []map[string]any{anthropicReply(`{"n": 3, "un`, "max_tokens", 100, 700)}}
	t.Setenv("ANTHROPIC_URL", api.serve(t).URL)
	t.Setenv("ANTHROPIC_API_KEY", "claude-key")
	prog := load(t, `
import ai

type Answer = { n: Int, unit: Unit }
type Unit = Items | Boxes

@budget(tokens: 800)
job ask() -> Result(Answer, ai.AiError) {
  ai.generate!(model: "claude-sonnet-5-5", prompt: "how many")
}
`)
	in := New(prog, io.Discard, io.Discard)
	if got := runJob(t, in, "ask"); got != "Failed(OverBudget)" {
		t.Errorf("ask = %s", got)
	}
}

func TestFunctionBudgetIsPerCall(t *testing.T) {
	noGateway(t)
	api := &standIn{replies: []map[string]any{chatReply(`{"n": 3, "unit": "Boxes"}`, "stop", 1500, 500)}}
	t.Setenv("OPENAI_URL", api.serve(t).URL)
	prog := load(t, `
import ai

type Answer = { n: Int, unit: Unit }
type Unit = Items | Boxes

@budget(tokens: 2_500)
fn count(text: String) -> Result(Answer, ai.AiError) uses {net} =
  ai.generate(model: "gpt-5.5", prompt: text)

job both() -> Result(Int, ai.AiError) {
  let a = count!("three boxes")?
  let b = count!("three more")?
  Ok(a.n + b.n)
}
`)
	in := New(prog, io.Discard, io.Discard)
	if got := runJob(t, in, "both"); got != "Completed(Ok(6))" {
		t.Errorf("both = %s", got)
	}
	if limit := api.requests[1]["max_completion_tokens"].(float64); limit < 2000 {
		t.Errorf("the second call's limit = %v: it has a budget of its own", limit)
	}
}

func TestUnknownPriceRefusesADollarBudget(t *testing.T) {
	noGateway(t)
	api := &standIn{replies: []map[string]any{chatReply(`{"n": 3, "unit": "Boxes"}`, "stop", 10, 5)}}
	t.Setenv("AI_URL", api.serve(t).URL)
	prog := load(t, `
import ai

type Answer = { n: Int, unit: Unit }
type Unit = Items | Boxes

@budget(usd: 1)
job ask() -> Result(Answer, ai.AiError) {
  ai.generate!(model: "house-model", prompt: "how many")
}
`)
	in := New(prog, io.Discard, io.Discard)
	if got := runJob(t, in, "ask"); !strings.Contains(got, "the price of house-model isn't known: give it in AI_PRICES") {
		t.Errorf("ask = %s", got)
	}
	if len(api.requests) != 0 {
		t.Errorf("%d requests, want none", len(api.requests))
	}
	// With its price given, it is asked.
	t.Setenv("AI_PRICES", "other=1/1, house-model=0.5/2")
	in = New(prog, io.Discard, io.Discard)
	if got := runJob(t, in, "ask"); got != "Completed(Ok({ n: 3, unit: Boxes }))" {
		t.Errorf("ask with AI_PRICES = %s", got)
	}
}


func TestRecordedUsageCountsInTests(t *testing.T) {
	prog := load(t, `
import ai

type Answer = { n: Int, unit: Unit }
type Unit = Items | Boxes

@budget(tokens: 1_000)
job twice() -> Result(Answer, ai.AiError) {
  let first: Answer = ai.generate!(model: "m", prompt: "how many")?
  ai.generate!(model: "m", prompt: "and now")
}
`)
	path := filepath.Join(t.TempDir(), "ai.json")
	if err := os.WriteFile(path, []byte(`{"contract": "ai", "exchanges": [
  {"op": "generate", "result": {"n": 3, "unit": "Items"}, "usage": {"input": 900, "output": 100}},
  {"op": "generate", "result": {"n": 4, "unit": "Items"}}
]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	in := New(prog, io.Discard, io.Discard)
	in.rt.aiReplies = in.loadReplies(path, ir.Pos{})
	// The first recorded reply spent the whole budget, so the second isn't
	// replayed.
	if got := runJob(t, in, "twice"); got != "Failed(OverBudget)" {
		t.Errorf("twice = %s", got)
	}
	if len(in.rt.aiReplies) != 1 {
		t.Errorf("%d recorded replies left, want 1", len(in.rt.aiReplies))
	}
}
