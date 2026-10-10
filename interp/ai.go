package interp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"eyelang/ir"
)

// hostGenerate is ai.generate (SPECS.md §19): it asks a model for a value of
// the type the call's result is wanted as. The schema comes from that type;
// a reply that doesn't match it is sent back once with the problem, and then
// is Err(Malformed(…)).
//
// The interpreter speaks the OpenAI-compatible chat completions protocol,
// which gateways such as the lab's offer for every model: AI_URL is the base
// URL (…/v1), and secret("ai-key") the key, if the gateway needs one. In
// tests, `with fakes(ai, from: "recordings/")` replays recorded replies.
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
	model, _ := params["model"].(string)
	system, _ := params["system"].(string)
	prompt, _ := params["prompt"].(string)
	messages := []map[string]string{}
	if system != "" {
		messages = append(messages, map[string]string{"role": "system", "content": system})
	}
	messages = append(messages, map[string]string{"role": "user", "content": prompt})

	var reply string
	var problem error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt == 1 {
			messages = append(messages,
				map[string]string{"role": "assistant", "content": reply},
				map[string]string{"role": "user", "content": "That reply doesn't match the schema: " + problem.Error() + ". Reply again with JSON that matches it."})
		}
		var status int
		var failure string
		reply, status, failure = in.complete(model, messages, schema, typeString(want), at)
		if failure != "" {
			return aiErr("ProviderError", int64(status), failure)
		}
		x, err := decodeJSON([]byte(reply))
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
	return aiErr("Malformed", reply, &List{Elems: []Value{problem.Error()}})
}

// complete sends one chat completion and returns the reply's text, or the
// status and message of a failure.
func (in *Interp) complete(model string, messages []map[string]string, schema map[string]any, name string, at ir.Pos) (string, int, string) {
	if in.rt.aiReplies != nil {
		if len(in.rt.aiReplies) == 0 {
			in.faultf(at, "the recordings for ai have no reply left for this call")
		}
		r := in.rt.aiReplies[0]
		in.rt.aiReplies = in.rt.aiReplies[1:]
		return string(r), 0, ""
	}
	base, ok := serviceURL("ai")
	if !ok {
		in.faultf(at, "ai.generate needs a model gateway: set AI_URL to its base URL, such as http://gateway:8317/v1")
	}
	body, _ := json.Marshal(map[string]any{
		"model":    model,
		"messages": messages,
		"response_format": map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   strings.NewReplacer("(", "_", ")", "", ", ", "_", ".", "_").Replace(name),
				"strict": true,
				"schema": schema,
			},
		},
	})
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	if key, ok := lookupSecret("ai-key"); ok {
		headers.Set("Authorization", "Bearer "+key)
	}
	status, data, err := doRequest("POST", base+"/chat/completions", headers, body)
	if err != nil {
		return "", 0, err.Error()
	}
	if status != 200 {
		return "", status, clipText(string(data), 1000)
	}
	var resp struct {
		Choices []struct {
			FinishReason string `json:"finish_reason"`
			Message      struct {
				Content string `json:"content"`
				Refusal string `json:"refusal"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &resp); err != nil || len(resp.Choices) == 0 {
		return "", status, "the gateway's answer has no choices"
	}
	c := resp.Choices[0]
	if c.Message.Refusal != "" {
		return "", status, "the model declined: " + c.Message.Refusal
	}
	if c.FinishReason != "stop" {
		return "", status, fmt.Sprintf("the reply didn't finish (finish_reason %q)", c.FinishReason)
	}
	return c.Message.Content, status, ""
}
