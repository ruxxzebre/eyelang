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
	schema, err := in.schemaOf(want, typeEnv{}, map[*ir.Type]bool{})
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

	var reply aiReply
	var problem error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt == 1 {
			req.messages = append(req.messages,
				aiMessage{"assistant", reply.text},
				aiMessage{"user", "That reply doesn't match the schema: " + problem.Error() + ". Reply again with JSON that matches it."})
		}
		var status int
		var failure string
		reply, status, failure = in.complete(req, at)
		in.charge(reply.spent)
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
		x, err := decodeJSON([]byte(reply.text))
		if err == nil && wrapped {
			obj, _ := x.(map[string]any)
			x = obj["value"]
		}
		var v Value
		if err == nil {
			v, err = in.fromWire(x, want, false)
		}
		if err == nil {
			return in.ok(v)
		}
		problem = err
	}
	return aiErr("Malformed", reply.text, &List{Elems: []Value{problem.Error()}})
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
	// maxTokens is how long the reply may be; capped says a budget set it.
	maxTokens int64
	capped    bool
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
	prompt, _ := json.Marshal([]any{req.system, req.messages, req.schema})
	// About three bytes a token, which counts English text on the high side.
	req.maxTokens, req.capped = in.allowance(req.model, int64(len(prompt))/3+1, at)
	if in.rt.aiReplies != nil {
		if len(in.rt.aiReplies) == 0 {
			in.faultf(at, "the recordings for ai have no reply left for this call")
		}
		r := in.rt.aiReplies[0]
		in.rt.aiReplies = in.rt.aiReplies[1:]
		return aiReply{text: string(r.Result), spent: r.spent(req.model)}, 0, ""
	}
	switch providerFor(req) {
	case "Anthropic":
		return in.completeAnthropic(req, at)
	case "OpenAI":
		base, ok := serviceURL("openai")
		if !ok {
			base = "https://api.openai.com/v1"
		}
		key, _ := lookupSecret("openai-api-key")
		return in.completeChat(req, base, key, "max_completion_tokens", at)
	}
	base, ok := serviceURL("ai")
	if !ok {
		in.faultf(at, "ai.generate needs a model gateway for %s: set AI_URL to its base URL, such as http://gateway:8317/v1, or name the provider with provider:", req.model)
	}
	key, _ := lookupSecret("ai-key")
	return in.completeChat(req, base, key, "max_tokens", at)
}

// completeChat speaks the chat completions protocol of OpenAI and the
// servers compatible with it, with structured output.
func (in *Interp) completeChat(req *aiRequest, base, key, maxField string, at ir.Pos) (aiReply, int, string) {
	messages := []aiMessage{}
	if req.system != "" {
		messages = append(messages, aiMessage{"system", req.system})
	}
	messages = append(messages, req.messages...)
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
				Content string `json:"content"`
				Refusal string `json:"refusal"`
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
	case c.FinishReason != "stop":
		return r, status, fmt.Sprintf("the reply didn't finish (finish_reason %q)", c.FinishReason)
	}
	r.text = c.Message.Content
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
	key, ok := lookupSecret("anthropic-api-key")
	if !ok {
		in.faultf(at, "ai.generate asks Anthropic for %s, and has no key: set ANTHROPIC_API_KEY, or put it in $EYE_SECRETS/anthropic-api-key", req.model)
	}
	body := map[string]any{
		"model":      req.model,
		"max_tokens": req.maxTokens,
		"messages":   req.messages,
		"output_config": map[string]any{
			"format": map[string]any{"type": "json_schema", "schema": req.schema},
		},
	}
	if req.system != "" {
		body["system"] = req.system
	}
	headers := http.Header{}
	headers.Set("x-api-key", key)
	headers.Set("anthropic-version", anthropicVersion)
	data, status, failure := in.postJSON(base+"/v1/messages", headers, body)
	if failure != "" {
		return aiReply{}, status, failure
	}
	var resp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
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
		if c.Type == "text" {
			r.text += c.Text
		}
	}
	switch resp.StopReason {
	case "end_turn", "stop_sequence":
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
// and message of anything else.
func (in *Interp) postJSON(target string, headers http.Header, body map[string]any) ([]byte, int, string) {
	data, _ := json.Marshal(body)
	headers.Set("Content-Type", "application/json")
	status, answer, err := in.doRequest("POST", target, headers, data)
	if err != nil {
		return nil, 0, err.Error()
	}
	if status != 200 {
		return nil, status, clipText(string(answer), 1000)
	}
	return answer, status, ""
}

// usage is what a call spent: its tokens, and their price, which the
// provider's own figure replaces when it gives one. A model whose price
// isn't known costs nothing here; a budget in dollars refuses to call it.
func usage(model string, in, out int64, cost *float64) spend {
	s := spend{Tokens: in + out}
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
