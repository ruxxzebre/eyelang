package interp

import (
	"io"
	"strings"
	"testing"
	"time"

	"eyelang/ir"
)

// eye runs shows a model call's step with the model, its tokens and price,
// and the bytes sent and received; and with the step's number, its whole
// reply.
func TestRunsShowAModelCall(t *testing.T) {
	noGateway(t)
	const reply = `{"n": 3, "unit": "Boxes"}`
	api := &standIn{replies: []map[string]any{anthropicReply(reply, "end_turn", 1000, 250)}}
	t.Setenv("ANTHROPIC_URL", api.serve(t).URL)
	t.Setenv("ANTHROPIC_API_KEY", "claude-key")
	prog := load(t, askClaude)
	in := New(prog, io.Discard, io.Discard)
	in.rt.startNow(prog.Root.Jobs["ask"], nil, nil, ir.Pos{})
	store := in.rt.index.Store

	var out strings.Builder
	FormatRuns(&out, Runs(prog, store), true, time.Now())
	// claude-sonnet-5-5 costs $2 in and $10 out per million tokens.
	want := "1. ai.generate → Ok({ n: 3, unit: Boxes })  [claude-sonnet-5-5: 1000 tokens in, 250 out, $0.0045; sent 17 B, received 25 B]"
	if !strings.Contains(out.String(), want) {
		t.Errorf("eye runs shows:\n%s\nwant a step\n%s", out.String(), want)
	}
	whole, err := StepValue(prog, store, 1, 1)
	if err != nil || whole != "Ok({ n: 3, unit: Boxes })" {
		t.Errorf("step 1 is %q, %v", whole, err)
	}
	if _, err := StepValue(prog, store, 1, 2); err == nil || !strings.Contains(err.Error(), "has 1 step, so it has no step 2") {
		t.Errorf("step 2: %v", err)
	}
}

// A reply that doesn't match its schema, twice, is shown as such: the
// problems, then the reply as the model wrote it.
func TestRunsShowAMalformedReply(t *testing.T) {
	noGateway(t)
	const reply = `{"n": "three", "unit": "Boxes"}`
	api := &standIn{replies: []map[string]any{anthropicReply(reply, "end_turn", 1000, 250)}}
	t.Setenv("ANTHROPIC_URL", api.serve(t).URL)
	t.Setenv("ANTHROPIC_API_KEY", "claude-key")
	prog := load(t, askClaude)
	in := New(prog, io.Discard, io.Discard)
	in.rt.startNow(prog.Root.Jobs["ask"], nil, nil, ir.Pos{})
	store := in.rt.index.Store

	var out strings.Builder
	FormatRuns(&out, Runs(prog, store), true, time.Now())
	if !strings.Contains(out.String(), "[claude-sonnet-5-5: 2000 tokens in, 500 out, $0.0090; sent ") ||
		!strings.Contains(out.String(), "received 62 B]") {
		t.Errorf("eye runs shows:\n%s\nwant both attempts counted", out.String())
	}
	whole, err := StepValue(prog, store, 1, 1)
	if err != nil || !strings.HasPrefix(whole, "The model's reply didn't match the schema it was asked for: ") ||
		!strings.HasSuffix(whole, "Its reply, as it wrote it:\n"+reply) {
		t.Errorf("step 1 is %q, %v", whole, err)
	}
}

func TestClipCutsOnACharacter(t *testing.T) {
	if got := clip("ééé", 3); got != "é… (4 B more)" {
		t.Errorf("clip = %q", got)
	}
	if got := clip("abc", 3); got != "abc" {
		t.Errorf("clip = %q", got)
	}
}
