package interp

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"eyelang/ir"
)

// A model's tools (SPECS.md §19): the functions passed to ai.generate as
// tools:. Each is sent with the call as a name, a description (its ##
// comment) and an input schema (its parameters' types). When a reply calls
// tools, each call's input is decoded into the parameters, the function
// runs, and its result goes back as JSON, until the model answers without
// calling one. That answer is decoded as a call without tools decodes it.
//
// In a job, ai.generate!(…) records every reply and every tool's result as
// a step of its own, so a run that restarts in the middle replays the
// conversation: the model isn't asked again and no tool runs twice. A
// @destructive tool runs only with an Approval, which the call's approve:
// function gets from a person, once for each of the model's calls.

// maxToolReplies is how many replies that call tools one model call serves
// before it gives up.
const maxToolReplies = 20

// toolDef is a tool as a model is told about it.
type toolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Schema      map[string]any `json:"input_schema"`
}

// toolCall is a model's call of a tool: an id the result names, the tool,
// and its input as JSON.
type toolCall struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// toolResult is what goes back to the model for one call: JSON, or an
// error that says why the function didn't run.
type toolResult struct {
	ID      string `json:"id"`
	Content string `json:"content"`
	IsError bool   `json:"is_error,omitempty"`
}

// aiTurn is one message of a conversation with tools: the user's prompt or
// a note, the model's reply with the tools it calls, or the tools' results.
type aiTurn struct {
	Role    string       `json:"role"` // "user" or "assistant"
	Text    string       `json:"text,omitempty"`
	Calls   []toolCall   `json:"calls,omitempty"`
	Results []toolResult `json:"results,omitempty"`
}

// savedReply is a reply as its step records it.
type savedReply struct {
	Text    string     `json:"text,omitempty"`
	Calls   []toolCall `json:"calls,omitempty"`
	Refusal string     `json:"refusal,omitempty"`
	Cut     bool       `json:"cut,omitempty"`
	Status  int        `json:"status,omitempty"`
	Failure string     `json:"failure,omitempty"`
}

// The names a call's steps are recorded under, which eye runs shows.
const replyStep = "ai.generate: reply"

func toolStep(name string) string { return "tool: " + name }

// hasTools reports whether a call of ai.generate, fn, is given any tools,
// by name or in their place.
func hasTools(fn *ir.Func, args []argVal) bool {
	for i, a := range args {
		named := a.name == "tools"
		if a.name == "" && i < len(fn.Params) {
			named = fn.Params[i].Name == "tools"
		}
		if l, ok := a.v.(*List); ok && named {
			return len(l.Elems) > 0
		}
	}
	return false
}

// generateSteps calls ai.generate with tools as a job's waiting call: the
// call records its own steps in run.
func (in *Interp) generateSteps(run *jobRun, fn *Closure, args []argVal, want *ir.TypeExpr, at ir.Pos) Value {
	prevWant, prevRun := in.want, in.toolRun
	in.want, in.toolRun = want, run
	defer func() { in.want, in.toolRun = prevWant, prevRun }()
	v, _ := in.apply(fn, args, at)
	return v
}

// generateWithTools holds the conversation of a model call with tools. run
// is the job run whose steps it records, or nil when the call isn't one of
// a job's waiting calls.
func (in *Interp) generateWithTools(req *aiRequest, prompt string, tools []Value, approve Value, run *jobRun,
	aiErr func(string, ...Value) Value, wrapped bool, want *ir.TypeExpr, at ir.Pos) Value {
	byName := map[string]*Closure{}
	for _, t := range tools {
		c, ok := t.(*Closure)
		if !ok || c.Fn.Body == nil {
			in.faultf(at, "a tool is a function declared with fn, and %s isn't one", show(t))
		}
		if _, dup := byName[c.Fn.Name]; dup {
			in.faultf(at, "two tools are named %s: a model tells its tools apart by name", c.Fn.Name)
		}
		byName[c.Fn.Name] = c
		def, err := in.toolDef(c.Fn)
		if err != nil {
			in.faultf(at, "%s can't be a tool: %v", c.Fn.Name, err)
		}
		req.tools = append(req.tools, def)
	}
	req.turns = []aiTurn{{Role: "user", Text: prompt}}
	retried, rounds := false, 0
	for {
		reply := in.toolReply(run, req, at)
		switch {
		case reply.Failure != "":
			return aiErr("ProviderError", int64(reply.Status), reply.Failure)
		case reply.Refusal != "":
			return aiErr("Declined", reply.Refusal)
		case reply.Cut && req.capped:
			runFault(at, "OverBudget", "the reply from %s reached the end of the budget before it finished", req.model)
		case reply.Cut:
			return aiErr("ProviderError", int64(reply.Status), fmt.Sprintf("the reply didn't finish in %d tokens", req.maxTokens))
		}
		if len(reply.Calls) == 0 {
			v, err := in.decodeReply(reply.Text, want, wrapped)
			switch {
			case err == nil:
				return in.ok(v)
			case retried:
				return aiErr("Malformed", reply.Text, &List{Elems: []Value{err.Error()}})
			}
			retried = true
			req.turns = append(req.turns, aiTurn{Role: "assistant", Text: reply.Text}, aiTurn{Role: "user", Text: mismatchNote(err)})
			continue
		}
		if rounds++; rounds > maxToolReplies {
			return aiErr("ProviderError", int64(reply.Status), fmt.Sprintf("the model called tools in %d replies without answering", maxToolReplies))
		}
		req.turns = append(req.turns, aiTurn{Role: "assistant", Text: reply.Text, Calls: reply.Calls})
		results := aiTurn{Role: "user"}
		for _, call := range reply.Calls {
			results.Results = append(results.Results, in.runTool(run, byName, approve, call, at))
		}
		req.turns = append(req.turns, results)
	}
}

// toolReply is the model's next reply: recorded, when run already took
// this step, or asked for, and recorded when there is a run.
func (in *Interp) toolReply(run *jobRun, req *aiRequest, at ir.Pos) savedReply {
	var saved savedReply
	if run != nil && run.cursor < len(run.Log) {
		in.charge(run.Log[run.cursor].spent())
		v, _ := in.replay(run, replyStep, at)
		text, _ := v.(string)
		if err := json.Unmarshal([]byte(text), &saved); err != nil {
			in.faultf(at, "the recorded reply of %s isn't one: %v", req.model, err)
		}
		return saved
	}
	prevSpend := in.stepSpend
	if run != nil {
		in.stepSpend = &spend{}
	}
	reply, status, failure := in.complete(req, at)
	in.charge(reply.spent)
	sent, _ := json.Marshal(req.turns)
	in.noteCall(req.model, req.options, len(req.system)+len(sent), len(reply.text))
	saved = savedReply{Text: reply.text, Calls: reply.calls, Refusal: reply.refusal, Cut: reply.cut, Status: status, Failure: failure}
	if run != nil {
		spent := *in.stepSpend
		in.stepSpend = prevSpend
		data, _ := json.Marshal(saved)
		in.call = replyStep
		in.recordSpend(run, string(data), spent, at)
	}
	return saved
}

// runTool runs the tool a model called and returns what goes back to it.
// Input that doesn't fit the tool goes back as an error, and nothing runs.
func (in *Interp) runTool(run *jobRun, byName map[string]*Closure, approve Value, call toolCall, at ir.Pos) toolResult {
	fail := func(format string, args ...any) toolResult {
		msg, _ := json.Marshal(map[string]string{"error": fmt.Sprintf(format, args...)})
		return toolResult{ID: call.ID, Content: string(msg), IsError: true}
	}
	tool := byName[call.Name]
	if tool == nil {
		var names []string
		for n := range byName {
			names = append(names, n)
		}
		sort.Strings(names)
		return fail("there is no tool %s; the tools are %s", call.Name, strings.Join(names, ", "))
	}
	args, err := in.toolArgs(tool.Fn, call.Input)
	if err != nil {
		return fail("the input doesn't fit %s: %v", call.Name, err)
	}
	if tool.Fn.Destructive {
		// A person approves this one call; their Approval is the argument
		// @destructive adds.
		d, _ := in.apply(approve, []argVal{{v: toolCallText(tool.Fn, args)}}, at)
		decision, _ := d.(*Variant)
		switch {
		case decision != nil && decision.V.Name == "Approved" && len(decision.Vals) == 1:
			args = append(args, argVal{name: "approval", v: decision.Vals[0]})
		case decision != nil && decision.V.Name == "Expired":
			return fail("nobody approved this call of %s in time, so it didn't run", call.Name)
		default:
			return fail("a person declined this call of %s, so it didn't run", call.Name)
		}
	}
	var v Value
	replayed := false
	if run != nil && run.cursor < len(run.Log) {
		v, replayed = in.replay(run, toolStep(call.Name), at)
	}
	if !replayed {
		v = in.callTool(tool, args, at)
		if run != nil {
			in.call = toolStep(call.Name)
			in.record(run, v, at)
		}
	}
	data, err := json.Marshal(in.toWire(v, false))
	if err != nil {
		in.faultf(at, "what %s returned can't go back to the model as JSON: %v", call.Name, err)
	}
	return toolResult{ID: call.ID, Content: string(data)}
}

// callTool runs a tool. Its call is one step of the job, so inside it the
// code runs as outside a job: an entity call is a transaction of its own,
// as in main or a test.
func (in *Interp) callTool(tool *Closure, args []argVal, at ir.Pos) Value {
	prev := in.run
	in.run = nil
	defer func() { in.run = prev }()
	v, _ := in.apply(tool, args, at)
	return v
}

// toolDef describes a tool to a model: its parameters, but the approval
// @destructive adds, as an object with a property each, required unless it
// has a default.
func (in *Interp) toolDef(fn *ir.Func) (toolDef, error) {
	props := map[string]any{}
	required := []any{}
	for _, p := range toolParams(fn) {
		s, err := in.schemaOf(p.Type, typeEnv{}, map[*ir.Type]bool{})
		if err != nil {
			return toolDef{}, fmt.Errorf("its parameter %s: %v", p.Name, err)
		}
		props[p.Name] = s
		if p.Default == nil {
			required = append(required, p.Name)
		}
	}
	desc := fn.Doc
	if desc == "" {
		desc = fn.Name
	}
	return toolDef{Name: fn.Name, Description: desc, Schema: map[string]any{
		"type": "object", "properties": props, "required": required, "additionalProperties": false,
	}}, nil
}

// toolParams are the parameters a model fills in.
func toolParams(fn *ir.Func) []*ir.Param {
	if fn.Destructive {
		return fn.Params[:len(fn.Params)-1]
	}
	return fn.Params
}

// toolArgs decodes a call's input into the tool's arguments. A parameter
// with a default may be left out.
func (in *Interp) toolArgs(fn *ir.Func, input json.RawMessage) ([]argVal, error) {
	if len(input) == 0 {
		input = json.RawMessage("{}")
	}
	x, err := decodeJSON(input)
	if err != nil {
		return nil, err
	}
	obj, ok := x.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected an object, found %s", describeJSON(x))
	}
	var args []argVal
	for _, p := range toolParams(fn) {
		raw, given := obj[p.Name]
		if !given && p.Default != nil {
			continue
		}
		v, err := in.decodeWire(raw, p.Type, false, typeEnv{}, p.Name)
		if err != nil {
			return nil, err
		}
		args = append(args, argVal{name: p.Name, v: v})
	}
	for name := range obj {
		if !slicesHasParam(toolParams(fn), name) {
			return nil, fmt.Errorf("%s has no parameter %s", fn.Name, name)
		}
	}
	return args, nil
}

func slicesHasParam(params []*ir.Param, name string) bool {
	for _, p := range params {
		if p.Name == name {
			return true
		}
	}
	return false
}

// toolCallText writes a model's call of a tool as Eyelang would, for the
// person asked to approve it: label(issue: 8, name: "duplicate").
func toolCallText(fn *ir.Func, args []argVal) string {
	var parts []string
	for _, a := range args {
		var b strings.Builder
		writeShow(&b, a.v, true)
		parts = append(parts, a.name+": "+b.String())
	}
	return fn.Name + "(" + strings.Join(parts, ", ") + ")"
}

// rawInput is a tool call's arguments as a provider sent them, kept as text
// when they aren't JSON, so the call goes back to the model as an error.
func rawInput(s string) json.RawMessage {
	if json.Valid([]byte(s)) {
		return json.RawMessage(s)
	}
	quoted, _ := json.Marshal(s)
	return quoted
}

// anthropicTools are the tools as the Messages API takes them.
func anthropicTools(tools []toolDef) []any {
	out := []any{}
	for _, t := range tools {
		out = append(out, map[string]any{"name": t.Name, "description": t.Description, "input_schema": t.Schema})
	}
	return out
}

// anthropicTurns is the conversation as the Messages API takes it: a reply
// that calls tools has a tool_use block for each, and their results go back
// as tool_result blocks in a user message.
func anthropicTurns(turns []aiTurn) []any {
	out := []any{}
	for _, t := range turns {
		switch {
		case len(t.Results) > 0:
			var blocks []any
			for _, r := range t.Results {
				blocks = append(blocks, map[string]any{"type": "tool_result", "tool_use_id": r.ID, "content": r.Content, "is_error": r.IsError})
			}
			out = append(out, map[string]any{"role": "user", "content": blocks})
		case len(t.Calls) > 0:
			var blocks []any
			if t.Text != "" {
				blocks = append(blocks, map[string]any{"type": "text", "text": t.Text})
			}
			for _, c := range t.Calls {
				blocks = append(blocks, map[string]any{"type": "tool_use", "id": c.ID, "name": c.Name, "input": c.Input})
			}
			out = append(out, map[string]any{"role": "assistant", "content": blocks})
		default:
			out = append(out, aiMessage{t.Role, t.Text})
		}
	}
	return out
}

// chatTools are the tools as chat completions take them: function tools.
func chatTools(tools []toolDef) []any {
	out := []any{}
	for _, t := range tools {
		out = append(out, map[string]any{"type": "function", "function": map[string]any{
			"name": t.Name, "description": t.Description, "parameters": t.Schema,
		}})
	}
	return out
}

// chatTurns is the conversation as chat completions take it: a reply that
// calls tools has tool_calls, and each result is a message of role tool.
func chatTurns(turns []aiTurn) []any {
	out := []any{}
	for _, t := range turns {
		switch {
		case len(t.Results) > 0:
			for _, r := range t.Results {
				out = append(out, map[string]any{"role": "tool", "tool_call_id": r.ID, "content": r.Content})
			}
		case len(t.Calls) > 0:
			var calls []any
			for _, c := range t.Calls {
				calls = append(calls, map[string]any{"id": c.ID, "type": "function", "function": map[string]any{
					"name": c.Name, "arguments": string(c.Input),
				}})
			}
			var content any
			if t.Text != "" {
				content = t.Text
			}
			out = append(out, map[string]any{"role": "assistant", "content": content, "tool_calls": calls})
		default:
			out = append(out, aiMessage{t.Role, t.Text})
		}
	}
	return out
}
