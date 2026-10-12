package check

import (
	"strings"
	"testing"
)

// github is a module whose operations the tools below call.
const github = `pub type Issue = { id: Int, title: String }
pub type Labelled = | Labelled

@port(3005)
contract GitHub {
  query search(phrase: String, limit: Int) -> List(Issue)

  @destructive
  command label(issue: Int, name: String) -> Labelled
}
`

// toolsHeader declares the tools of SPECS.md §19's example.
const toolsHeader = `import ai
import github.{Issue, Labelled}

## Find open issues that mention a phrase.
fn searchIssues(phrase: String, limit: Int = 10) -> List(Issue) uses {net GitHub.search} {
  github.search(phrase, limit)
}

## Add a label to an issue.
@destructive
fn label(issue: Int, name: String) -> String uses {net GitHub.label} {
  match github.label(issue, name, approval) {
    Labelled => "labelled"
    Uncertain(_) => "not known"
  }
}

type Triage = { duplicateOf: Option(Int), labels: List(String) }
`

// TestModelTools checks functions passed to ai.generate as tools (SPECS.md
// §19): a named function converts to ai.Tool, its effects count where it is
// named, and @destructive adds the approval a person gives for each call.
// Each case lists every diagnostic the program has, exactly.
func TestModelTools(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string
	}{
		{"tools of different types go in one list, in a job and in a function that lists their effects", `
job triage(report: String) -> Result(Triage, ai.AiError) {
  ai.generate!(model: "claude-opus-5-5", prompt: report, tools: [searchIssues, label],
    approve: fn call: approval!(call, from: "maintainer", within: 1 day))
}

pub fn ask(report: String) -> Result(Triage, ai.AiError) uses {net Ai.generate, net GitHub.search} =
  ai.generate(model: "claude-opus-5-5", prompt: report, tools: [searchIssues])

pub fn askAny(report: String) -> Result(Triage, ai.AiError) uses {net} =
  ai.generate(model: "claude-opus-5-5", prompt: report, tools: [searchIssues, label], approve: fn _: Declined)`, nil},
		{"a public function that passes label lists net GitHub.label", `
pub fn ask(report: String) -> Result(Triage, ai.AiError) uses {net Ai.generate, net GitHub.search} =
  ai.generate(model: "claude-opus-5-5", prompt: report, tools: [searchIssues, label], approve: fn _: Declined)`, []string{
			"main.eye:20:1: ask uses net GitHub.label, which its uses clause doesn't list: run eye fmt, or add it",
		}},
		{"a call that names a @destructive tool says who approves its calls", `
job triage(report: String) -> Result(Triage, ai.AiError) {
  ai.generate!(model: "claude-opus-5-5", prompt: report, tools: [searchIssues, label])
}`, []string{
			"main.eye:21:80: label is @destructive, so each call the model makes of it needs a person's approval: give ai.generate approve:, as approve: fn call: approval!(call, from: someone, within: 1 day)",
		}},
		{"a tool is a function declared with fn, with written types that have Schema", `
fn untyped(x) = x + 1
fn generic(x: a) -> a = x
## Reads a secret.
fn token(name: String) -> Secret(String) uses {secret} = secret(name)
fn holds(s: Secret(String)) -> Int = 1
## Waits.
fn waits(n: Int) {
  sleep!(1 s)
}

fn triage(report: String, f: fn(String) -> Int) -> Result(Triage, ai.AiError) =
  ai.generate(model: "m", prompt: report, tools: [fn s: 1, f, ai.generate, untyped, generic, token, holds, waits])`, []string{
			"main.eye:31:51: a tool is a function declared with fn and named here, as tools: [searchIssues], since its name, its ## comment and its parameters' types describe it to the model; a lambda or a function passed in has none of them",
			"main.eye:31:60: a tool is a function declared with fn and named here, as tools: [searchIssues], since its name, its ## comment and its parameters' types describe it to the model; a lambda or a function passed in has none of them",
			"main.eye:31:63: generate is an extern fn, and a tool is a function declared with fn, whose ## comment and parameters describe it to the model: wrap it in one",
			"main.eye:31:76: untyped's parameter x has no written type, and a tool's input schema comes from its parameters' types: write it, as x: String",
			"main.eye:31:85: generic is generic, and a tool has one input schema: give its parameter x a type without type variables",
			"main.eye:31:85: generic is generic, and what a tool returns goes back to the model as JSON: give its result a type without type variables",
			"main.eye:31:94: what token returns, which goes back to the model as a tool's result, needs Schema, and Secret(String) has no Schema: a Secret can't be shown, saved or compared, so it never leaks",
			"main.eye:31:101: holds's parameter s, which the model fills in as a tool's input, needs Schema, and Secret(String) has no Schema: a Secret can't be shown, saved or compared, so it never leaks",
			"main.eye:31:108: waits waits, and a tool runs inside its model call, which can't pause for it: pass a function that doesn't wait (a @destructive tool has the runtime ask a person for each call)",
		}},
		{"@destructive adds approval: Approval, which code passes from approval!", `
fn direct(ok: Approval) {
  let _ = label(1, "bug", ok)
  let _ = label(1, "bug")
}`, []string{
			"main.eye:22:11: label needs an argument for approval: label is @destructive, so it takes the Approval that approval!(…) gives",
		}},
		{"a function can't declare the approval @destructive adds", `
@destructive
fn wipe(id: Int, approval: String) {}`, []string{
			"main.eye:20:1: @destructive adds the parameter approval, so wipe can't declare one of that name",
		}},
		{"a Tool can't be saved", `
job keep(tools: List(ai.Tool)) {}`, []string{
			"main.eye:20:1: keep's argument tools is a Tool, which can't be saved, and a job's arguments are",
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := diagnoseWith(t, toolsHeader+c.src, map[string]string{"github/github.eye": github})
			if strings.Join(got, "\n") != strings.Join(c.want, "\n") {
				t.Errorf("diagnostics:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(c.want, "\n"))
			}
		})
	}
}
