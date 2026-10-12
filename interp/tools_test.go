package interp

import (
	"encoding/json"
	"io"
	"strings"
	"testing"
)

// askWithTools asks a model that may call two tools.
const askWithTools = `
import ai

type Answer = { n: Int, unit: Unit }
type Unit = Items | Boxes

## Counts the boxes on a shelf.
fn countBoxes(shelf: String, max: Int = 100) -> Int = if shelf == "a" then 3 else 0

## Says what a box holds.
fn contents(box: Int) -> List(String) = ["tea"]

job ask(model: String) -> Result(Answer, ai.AiError) {
  ai.generate!(model:, system: "Count.", prompt: "how many on shelf a", tools: [countBoxes, contents])
}
`

// toolSteps are the calls a run's steps record.
func toolSteps(t *testing.T, in *Interp) []string {
	t.Helper()
	data, _ := in.rt.store.Run(1)
	var calls []string
	for _, s := range decodeRun(data).Log {
		calls = append(calls, s.Call)
	}
	return calls
}

// jsonOf writes v as JSON, for comparing parts of a request.
func jsonOf(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}

func TestAnthropicCallsTools(t *testing.T) {
	noGateway(t)
	api := &standIn{replies: []map[string]any{
		{
			"type": "message", "role": "assistant", "stop_reason": "tool_use",
			"content": []any{
				map[string]any{"type": "text", "text": "Let me look."},
				map[string]any{"type": "tool_use", "id": "toolu_1", "name": "countBoxes", "input": map[string]any{"shelf": "a"}},
				map[string]any{"type": "tool_use", "id": "toolu_2", "name": "contents", "input": map[string]any{"box": 1}},
			},
			"usage": map[string]any{"input_tokens": 300, "output_tokens": 40},
		},
		anthropicReply(`{"n": 3, "unit": "Boxes"}`, "end_turn", 400, 20),
	}}
	t.Setenv("ANTHROPIC_URL", api.serve(t).URL)
	t.Setenv("ANTHROPIC_API_KEY", "claude-key")
	in := New(load(t, askWithTools), io.Discard, io.Discard)
	if got := runJob(t, in, "ask", "claude-sonnet-5-5"); got != "Completed(Ok({ n: 3, unit: Boxes }))" {
		t.Fatalf("ask = %s", got)
	}
	if len(api.requests) != 2 {
		t.Fatalf("%d requests, want 2", len(api.requests))
	}
	// The tools go with every request, described by their ## comments and
	// parameters; a parameter with a default isn't required.
	wantTools := `[{"description":"Counts the boxes on a shelf.","input_schema":{"additionalProperties":false,"properties":{"max":{"type":"integer"},"shelf":{"type":"string"}},"required":["shelf"],"type":"object"},"name":"countBoxes"},` +
		`{"description":"Says what a box holds.","input_schema":{"additionalProperties":false,"properties":{"box":{"type":"integer"}},"required":["box"],"type":"object"},"name":"contents"}]`
	for i, r := range api.requests {
		if got := jsonOf(r["tools"]); got != wantTools {
			t.Errorf("request %d's tools = %s", i+1, got)
		}
	}
	// The second request has the model's calls and what each tool returned.
	msgs := api.requests[1]["messages"].([]any)
	if len(msgs) != 3 {
		t.Fatalf("the second request's messages = %s", jsonOf(msgs))
	}
	if got, want := jsonOf(msgs[1]), `{"content":[{"text":"Let me look.","type":"text"},{"id":"toolu_1","input":{"shelf":"a"},"name":"countBoxes","type":"tool_use"},{"id":"toolu_2","input":{"box":1},"name":"contents","type":"tool_use"}],"role":"assistant"}`; got != want {
		t.Errorf("the model's calls went back as %s", got)
	}
	if got, want := jsonOf(msgs[2]), `{"content":[{"content":"3","is_error":false,"tool_use_id":"toolu_1","type":"tool_result"},{"content":"[\"tea\"]","is_error":false,"tool_use_id":"toolu_2","type":"tool_result"}],"role":"user"}`; got != want {
		t.Errorf("the tools' results went back as %s", got)
	}
	// Each reply and each tool's result is a step.
	if got := strings.Join(toolSteps(t, in), "; "); got != "ai.generate: reply; tool: countBoxes; tool: contents; ai.generate: reply" {
		t.Errorf("steps = %s", got)
	}
}

func TestChatCompletionsCallTools(t *testing.T) {
	noGateway(t)
	api := &standIn{replies: []map[string]any{
		{
			"choices": []any{map[string]any{"finish_reason": "tool_calls", "message": map[string]any{
				"content": nil,
				"tool_calls": []any{
					map[string]any{"id": "call_a", "type": "function", "function": map[string]any{"name": "countBoxes", "arguments": `{"shelf":"a"}`}},
					map[string]any{"id": "call_b", "type": "function", "function": map[string]any{"name": "contents", "arguments": `{"box":1}`}},
				},
			}}},
			"usage": map[string]any{"prompt_tokens": 300, "completion_tokens": 40},
		},
		chatReply(`{"n": 3, "unit": "Boxes"}`, "stop", 400, 20),
	}}
	t.Setenv("OPENAI_URL", api.serve(t).URL)
	in := New(load(t, askWithTools), io.Discard, io.Discard)
	if got := runJob(t, in, "ask", "gpt-5.5"); got != "Completed(Ok({ n: 3, unit: Boxes }))" {
		t.Fatalf("ask = %s", got)
	}
	if len(api.requests) != 2 {
		t.Fatalf("%d requests, want 2", len(api.requests))
	}
	tools := api.requests[0]["tools"].([]any)
	if got, want := jsonOf(tools[1]), `{"function":{"description":"Says what a box holds.","name":"contents","parameters":{"additionalProperties":false,"properties":{"box":{"type":"integer"}},"required":["box"],"type":"object"}},"type":"function"}`; got != want {
		t.Errorf("a tool was sent as %s", got)
	}
	msgs := api.requests[1]["messages"].([]any)
	if got, want := jsonOf(msgs[2:]), `[{"content":null,"role":"assistant","tool_calls":[{"function":{"arguments":"{\"shelf\":\"a\"}","name":"countBoxes"},"id":"call_a","type":"function"},{"function":{"arguments":"{\"box\":1}","name":"contents"},"id":"call_b","type":"function"}]},`+
		`{"content":"3","role":"tool","tool_call_id":"call_a"},{"content":"[\"tea\"]","role":"tool","tool_call_id":"call_b"}]`; got != want {
		t.Errorf("the second request's messages end with %s", got)
	}
}

// A call that doesn't fit its tool goes back to the model as an error, and
// no function runs.
func TestABadToolCallGoesBackAsAnError(t *testing.T) {
	noGateway(t)
	calls := func(name string, input any) map[string]any {
		return map[string]any{
			"type": "message", "role": "assistant", "stop_reason": "tool_use",
			"content": []any{map[string]any{"type": "tool_use", "id": "toolu_1", "name": name, "input": input}},
		}
	}
	api := &standIn{replies: []map[string]any{
		calls("countBoxes", map[string]any{"shelf": 5}),
		calls("weigh", map[string]any{}),
		calls("contents", map[string]any{"box": 1, "colour": "red"}),
		anthropicReply(`{"n": 0, "unit": "Items"}`, "end_turn", 10, 5),
	}}
	t.Setenv("ANTHROPIC_URL", api.serve(t).URL)
	t.Setenv("ANTHROPIC_API_KEY", "claude-key")
	in := New(load(t, askWithTools), io.Discard, io.Discard)
	if got := runJob(t, in, "ask", "claude-sonnet-5-5"); got != "Completed(Ok({ n: 0, unit: Items }))" {
		t.Fatalf("ask = %s", got)
	}
	for i, want := range []string{
		`the input doesn't fit countBoxes: shelf: expected a string, found 5`,
		`there is no tool weigh; the tools are contents, countBoxes`,
		`the input doesn't fit contents: contents has no parameter colour`,
	} {
		msgs := api.requests[i+1]["messages"].([]any)
		result := msgs[len(msgs)-1].(map[string]any)["content"].([]any)[0].(map[string]any)
		if result["is_error"] != true || !strings.Contains(result["content"].(string), want) {
			t.Errorf("result %d = %s, want an error that says %q", i+1, jsonOf(result), want)
		}
	}
	if got := strings.Join(toolSteps(t, in), "; "); strings.Contains(got, "tool:") {
		t.Errorf("a tool ran: steps = %s", got)
	}
}

// A model that keeps calling tools is stopped.
func TestToolCallsHaveALimit(t *testing.T) {
	noGateway(t)
	api := &standIn{replies: []map[string]any{{
		"type": "message", "role": "assistant", "stop_reason": "tool_use",
		"content": []any{map[string]any{"type": "tool_use", "id": "toolu_1", "name": "contents", "input": map[string]any{"box": 1}}},
	}}}
	t.Setenv("ANTHROPIC_URL", api.serve(t).URL)
	t.Setenv("ANTHROPIC_API_KEY", "claude-key")
	in := New(load(t, askWithTools), io.Discard, io.Discard)
	if got := runJob(t, in, "ask", "claude-sonnet-5-5"); got != `Completed(Err(ProviderError(status: 200, message: "the model called tools in 20 replies without answering")))` {
		t.Fatalf("ask = %s", got)
	}
	if len(api.requests) != maxToolReplies+1 {
		t.Errorf("%d requests, want %d", len(api.requests), maxToolReplies+1)
	}
}
