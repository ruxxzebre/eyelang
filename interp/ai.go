package interp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"eyelang/ir"
)

// hostGenerate is ai.generate (SPECS.md §19): it asks a model for a value of
// the type the call's result is wanted as. The schema comes from that type;
// a reply that doesn't match it is sent back once with the problem, and then
// is Err(Malformed(…)).
//
// Three providers answer (ai.Provider): Anthropic's Messages API, OpenAI's
// chat completions, and an OpenAI-compatible gateway at AI_URL, which serves
// every model when it is set. Every reply's tokens and price count against
// the @budgets being kept (budget.go). In tests, `with fakes(ai, from:
// "recordings/")` replays recorded replies.
func hostGenerate(in *Interp, params map[string]Value, at ir.Pos) Value {
	want := in.want
	if want == nil {
		in.faultf(at, "ai.generate needs to know the type to ask for: write `let x: T = ai.generate!(…)`, or return it from a function that says its type")
	}
	schema, err := in.schemaOf(want, typeEnv{}, map[string]bool{})
	if err != nil {
		in.faultf(at, "ai.generate can't ask for %s: %v", typeString(want), err)
	}
	// Providers want an object at the top; anything else is wrapped in one.
	wrapped := schema["type"] != "object"
	if wrapped {
		schema = map[string]any{"type": "object", "properties": map[string]any{"value": schema},
			"required": []any{"value"}, "additionalProperties": false}
	}
	aiMod := in.stdModule("ai", at)
	aiErr := func(variant string, vals ...Value) Value {
		for _, v := range aiMod.Types["AiError"].Variants {
			if v.Name == variant {
				return in.errV(&Variant{V: v, Vals: vals})
			}
		}
		panic("ai.AiError has no variant " + variant)
	}
	req := &aiRequest{name: typeString(want), schema: schema}
	req.model, _ = params["model"].(string)
	req.system, _ = params["system"].(string)
	prompt, _ := params["prompt"].(string)
	req.messages = []aiMessage{{"user", prompt}}
	if p, ok := params["provider"].(*Variant); ok && p.V == in.pre.some {
		req.provider = p.Vals[0].(*Variant).V.Name
	}
	if e, ok := params["effort"].(*Variant); ok && e.V == in.pre.some {
		req.effort = strings.ToLower(e.Vals[0].(*Variant).V.Name)
	}
	// A call written with ! in a job records each reply and each tool's
	// result as a step of its own; durable says so.
	run := in.toolRun
	in.toolRun = nil
	if tools, _ := params["tools"].(*List); tools != nil && len(tools.Elems) > 0 {
		return in.generateWithTools(req, prompt, tools.Elems, params["approve"], run, aiErr, wrapped, want, at)
	}

	var reply aiReply
	var problem error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt == 1 {
			req.messages = append(req.messages,
				aiMessage{"assistant", reply.text},
				aiMessage{"user", mismatchNote(problem)})
		}
		var status int
		var failure string
		reply, status, failure = in.complete(req, at)
		in.charge(reply.spent)
		sent := len(req.system)
		for _, m := range req.messages {
			sent += len(m.Content)
		}
		in.noteCall(req.model, req.options, sent, len(reply.text))
		switch {
		case failure != "":
			return aiErr("ProviderError", int64(status), failure)
		case reply.refusal != "":
			return aiErr("Declined", reply.refusal)
		case reply.cut && req.capped:
			runFault(at, "OverBudget", "the reply from %s reached the end of the budget before it finished", req.model)
		case reply.cut:
			return aiErr("ProviderError", int64(status), fmt.Sprintf("the reply didn't finish in %d tokens", req.maxTokens))
		}
		v, err := in.decodeReply(reply.text, want, wrapped)
		if err == nil {
			return in.ok(v)
		}
		problem = err
	}
	return aiErr("Malformed", reply.text, &List{Elems: []Value{problem.Error()}})
}

// decodeReply reads a model's answer as the type asked for; wrapped says
// the schema put it in an object's "value".
func (in *Interp) decodeReply(text string, want *ir.TypeExpr, wrapped bool) (Value, error) {
	x, err := decodeJSON([]byte(text))
	if err != nil {
		return nil, err
	}
	if wrapped {
		obj, _ := x.(map[string]any)
		x = obj["value"]
	}
	return in.fromWire(x, want, false)
}

// mismatchNote is what goes back to a model whose answer didn't match the
// schema, before it is asked once more.
func mismatchNote(problem error) string {
	return "That reply doesn't match the schema: " + problem.Error() + ". Reply again with JSON that matches it."
}

// aiRequest is one model call: the conversation so far and the schema the
// reply must match.
type aiRequest struct {
	provider string // "Anthropic", "OpenAI" or "Gateway"; "" lets the model's name decide
	model    string
	system   string
	messages []aiMessage
	schema   map[string]any
	name     string // the type asked for, which names the schema
	// tools are the functions the model may call, and turns the
	// conversation, which replaces messages when there are tools
	// (tools.go).
	tools []toolDef
	turns []aiTurn
	// effort is how much the model reasons: "low", "medium" or "high"; ""
	// leaves the provider's default.
	effort string
	// maxTokens is how long the reply may be; capped says a budget set it.
	maxTokens int64
	capped    bool
	// options says how the call was sent: the API, how it asked for the
	// schema, and the settings it gave, for eye runs.
	options string
}

type aiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// aiReply is what a provider answered.
type aiReply struct {
	text    string
	refusal string // why the model declined, if it did
	cut     bool   // the reply stopped at maxTokens
	calls   []toolCall
	spent   spend
}

// openAIModel matches the model names OpenAI serves itself.
var openAIModel = regexp.MustCompile(`^(gpt-|o\d)`)

// providerFor decides who answers: the call's provider:, else a gateway when
// AI_URL is set, else the model's name.
func providerFor(req *aiRequest) string {
	if req.provider != "" {
		return req.provider
	}
	if _, ok := serviceURL("ai"); ok {
		return "Gateway"
	}
	switch {
	case strings.HasPrefix(req.model, "claude-"):
		return "Anthropic"
	case openAIModel.MatchString(req.model):
		return "OpenAI"
	}
	return "Gateway"
}

// complete sends one model call and returns the reply, or the status and
// message of a failure. It ends the run as Failed(OverBudget), without
// sending anything, when the call doesn't fit the budgets being kept.
func (in *Interp) complete(req *aiRequest, at ir.Pos) (aiReply, int, string) {
	prompt, _ := json.Marshal([]any{req.system, req.messages, req.turns, req.tools, req.schema})
	// About three bytes a token, which counts English text on the high side.
	req.maxTokens, req.capped = in.allowance(req.model, int64(len(prompt))/3+1, at)
	if in.rt.aiReplies != nil {
		if len(in.rt.aiReplies) == 0 {
			in.faultf(at, "the recordings for ai have no reply left for this call")
		}
		r := in.rt.aiReplies[0]
		in.rt.aiReplies = in.rt.aiReplies[1:]
		req.options = "a recorded reply, from with fakes(ai, …)"
		reply := aiReply{text: string(r.Result), spent: r.spent(req.model)}
		for i, c := range r.Calls {
			if req.tools == nil {
				in.faultf(at, "the recorded reply calls the tool %s, and this call has no tools", c.Tool)
			}
			reply.calls = append(reply.calls, toolCall{ID: fmt.Sprintf("call-%d-%d", len(req.turns), i+1), Name: c.Tool, Input: c.Input})
		}
		return reply, 0, ""
	}
	in.rt.offline(in.stdModule("ai", at), "ai.generate", at)
	reply, status, failure := in.ask(req, at)
	in.rt.recordReply(reply, failure)
	return reply, status, failure
}

// ask sends one model call to its provider.
func (in *Interp) ask(req *aiRequest, at ir.Pos) (aiReply, int, string) {
	switch providerFor(req) {
	case "Anthropic":
		return in.completeAnthropic(req, at)
	case "OpenAI":
		base, ok := serviceURL("openai")
		if !ok {
			base = "https://api.openai.com/v1"
		}
		key, ok := in.lookupSecret("openai-api-key")
		if !ok && in.rt.liveAI() {
			in.rt.skipTest(at, "--live ai: ai.generate asks OpenAI for %s, and OPENAI_API_KEY isn't set", req.model)
		}
		return in.completeChat(req, base, key, "max_completion_tokens", at)
	}
	base, ok := serviceURL("ai")
	if !ok {
		if in.rt.liveAI() {
			in.rt.skipTest(at, "--live ai: ai.generate needs a model gateway for %s, and AI_URL isn't set", req.model)
		}
		in.faultf(at, "ai.generate needs a model gateway for %s: set AI_URL to its base URL, such as http://gateway:8317/v1, or name the provider with provider:", req.model)
	}
	key, _ := in.lookupSecret("ai-key")
	return in.completeChat(req, base, key, "max_tokens", at)
}

// completeChat speaks the chat completions protocol of OpenAI and the
// servers compatible with it, with structured output.
func (in *Interp) completeChat(req *aiRequest, base, key, maxField string, at ir.Pos) (aiReply, int, string) {
	messages := []any{}
	if req.system != "" {
		messages = append(messages, aiMessage{"system", req.system})
	}
	if req.turns != nil {
		messages = append(messages, chatTurns(req.turns)...)
	} else {
		for _, m := range req.messages {
			messages = append(messages, m)
		}
	}
	body := map[string]any{
		"model":    req.model,
		"messages": messages,
		"response_format": map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   schemaName(req.name),
				"strict": true,
				"schema": req.schema,
			},
		},
	}
	if req.capped {
		body[maxField] = req.maxTokens
	}
	if req.effort != "" {
		body["reasoning_effort"] = req.effort
	}
	if req.tools != nil {
		body["tools"] = chatTools(req.tools)
	}
	via := "OpenAI's chat completions"
	if maxField == "max_tokens" {
		via = "chat completions at AI_URL"
	}
	req.options = via + ": response_format json_schema, strict"
	if req.tools != nil {
		req.options += fmt.Sprintf(", %d function tools", len(req.tools))
	}
	if req.capped {
		req.options += fmt.Sprintf(", %s %d", maxField, req.maxTokens)
	}
	if req.effort != "" {
		req.options += ", reasoning_effort " + req.effort
	} else {
		req.options += "; no reasoning_effort or other reasoning settings"
	}
	headers := http.Header{}
	if key != "" {
		headers.Set("Authorization", "Bearer "+key)
	}
	data, status, failure := in.postJSON(base+"/chat/completions", headers, body)
	if failure != "" {
		return aiReply{}, status, failure
	}
	var resp struct {
		Choices []struct {
			FinishReason string `json:"finish_reason"`
			Message      struct {
				Content   string `json:"content"`
				Refusal   string `json:"refusal"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int64    `json:"prompt_tokens"`
			CompletionTokens int64    `json:"completion_tokens"`
			Cost             *float64 `json:"cost"` // gateways such as OpenRouter's say what a call cost
		} `json:"usage"`
	}
	if err := json.Unmarshal(data, &resp); err != nil || len(resp.Choices) == 0 {
		return aiReply{}, status, "the answer has no choices"
	}
	r := aiReply{spent: usage(req.model, resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.Cost)}
	c := resp.Choices[0]
	switch {
	case c.Message.Refusal != "":
		r.refusal = c.Message.Refusal
	case c.FinishReason == "length":
		r.cut = true
	case c.FinishReason == "tool_calls" && req.tools != nil:
	case c.FinishReason != "stop":
		return r, status, fmt.Sprintf("the reply didn't finish (finish_reason %q)", c.FinishReason)
	}
	r.text = c.Message.Content
	for _, tc := range c.Message.ToolCalls {
		r.calls = append(r.calls, toolCall{ID: tc.ID, Name: tc.Function.Name, Input: rawInput(tc.Function.Arguments)})
	}
	return r, status, ""
}

// anthropicVersion is the version of Anthropic's API the requests are
// written for.
const anthropicVersion = "2023-06-01"

// completeAnthropic speaks Anthropic's Messages API, with structured output.
func (in *Interp) completeAnthropic(req *aiRequest, at ir.Pos) (aiReply, int, string) {
	base, ok := serviceURL("anthropic")
	if !ok {
		base = "https://api.anthropic.com"
	}
	key, ok := in.lookupSecret("anthropic-api-key")
	if !ok {
		if in.rt.liveAI() {
			in.rt.skipTest(at, "--live ai: ai.generate asks Anthropic for %s, and ANTHROPIC_API_KEY isn't set", req.model)
		}
		in.faultf(at, "ai.generate asks Anthropic for %s, and has no key: set ANTHROPIC_API_KEY, or put it in $EYE_SECRETS/anthropic-api-key", req.model)
	}
	output := map[string]any{"format": map[string]any{"type": "json_schema", "schema": req.schema}}
	if req.effort != "" {
		output["effort"] = req.effort
	}
	body := map[string]any{
		"model":         req.model,
		"max_tokens":    req.maxTokens,
		"messages":      req.messages,
		"output_config": output,
	}
	if req.system != "" {
		body["system"] = req.system
	}
	tools := ""
	if req.turns != nil {
		body["messages"] = anthropicTurns(req.turns)
		body["tools"] = anthropicTools(req.tools)
		tools = fmt.Sprintf(", %d tools", len(req.tools))
	}
	effort := "; no thinking settings"
	if req.effort != "" {
		effort = ", output_config.effort " + req.effort
	}
	req.options = fmt.Sprintf("Anthropic's Messages API: output_config.format json_schema%s, max_tokens %d%s", tools, req.maxTokens, effort)
	headers := http.Header{}
	headers.Set("x-api-key", key)
	headers.Set("anthropic-version", anthropicVersion)
	data, status, failure := in.postJSON(base+"/v1/messages", headers, body)
	if failure != "" {
		return aiReply{}, status, failure
	}
	var resp struct {
		Content []struct {
			Type  string          `json:"type"`
			Text  string          `json:"text"`
			ID    string          `json:"id"`    // a tool_use block's
			Name  string          `json:"name"`  // the tool it calls
			Input json.RawMessage `json:"input"` // and its input
		} `json:"content"`
		StopReason  string `json:"stop_reason"`
		StopDetails *struct {
			Category    string `json:"category"`
			Explanation string `json:"explanation"`
		} `json:"stop_details"`
		Usage struct {
			InputTokens              int64 `json:"input_tokens"`
			OutputTokens             int64 `json:"output_tokens"`
			CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
			CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return aiReply{}, status, "the answer isn't a message: " + err.Error()
	}
	u := resp.Usage
	r := aiReply{spent: usage(req.model, u.InputTokens+u.CacheCreationInputTokens+u.CacheReadInputTokens, u.OutputTokens, nil)}
	for _, c := range resp.Content {
		switch c.Type {
		case "text":
			r.text += c.Text
		case "tool_use":
			r.calls = append(r.calls, toolCall{ID: c.ID, Name: c.Name, Input: c.Input})
		}
	}
	switch resp.StopReason {
	case "end_turn", "stop_sequence":
	case "tool_use":
		if req.tools == nil {
			return r, status, "the model called a tool, and the call has none"
		}
	case "max_tokens":
		r.cut = true
	case "refusal":
		r.refusal = "the model declined"
		if d := resp.StopDetails; d != nil && d.Explanation != "" {
			r.refusal = d.Explanation
		} else if d != nil && d.Category != "" {
			r.refusal += " (" + d.Category + ")"
		}
	default:
		return r, status, fmt.Sprintf("the reply didn't finish (stop_reason %q)", resp.StopReason)
	}
	return r, status, ""
}

// postJSON posts body as JSON and returns the answer of a 200, or the status
// and message of anything else. A request that never reached the provider,
// and one it answered with 429 or a 5xx, which did nothing, are sent again
// for about a minute, as a contract's query is; a request that went out and
// had no answer isn't, since the provider may have done the work and
// charged for it.
func (in *Interp) postJSON(target string, headers http.Header, body map[string]any) ([]byte, int, string) {
	data, _ := json.Marshal(body)
	headers.Set("Content-Type", "application/json")
	for try := 0; ; try++ {
		status, answer, err := in.doRequest("POST", target, headers, data)
		switch {
		case err != nil && notSent(err) && in.tryAgain(try):
		case err != nil:
			return nil, 0, whyNoAnswer(target, "", err, try+1)
		case (status == 429 || status >= 500) && in.tryAgain(try):
		case status != 200:
			if try > 0 {
				return nil, status, fmt.Sprintf("%s, %d times over %s", clipText(strings.TrimSpace(string(answer)), 1000), try+1, triedFor(try+1))
			}
			return nil, status, clipText(string(answer), 1000)
		default:
			return answer, status, ""
		}
	}
}

// usage is what a call spent: its tokens, and their price, which the
// provider's own figure replaces when it gives one. A model whose price
// isn't known costs nothing here; a budget in dollars refuses to call it.
func usage(model string, in, out int64, cost *float64) spend {
	s := spend{Tokens: in + out, In: in, Out: out}
	if cost != nil {
		s.USD = *cost
	} else if p, ok := priceOf(model); ok {
		s.USD = p.cost(in, out)
	}
	return s
}

// schemaName is the type's name as providers accept a schema's name.
func schemaName(name string) string {
	return strings.NewReplacer("(", "_", ")", "", ", ", "_", ".", "_").Replace(name)
}
