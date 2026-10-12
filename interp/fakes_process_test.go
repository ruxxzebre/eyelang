package interp

import (
	"strings"
	"testing"
)

const processProgram = `import process

fn pull() -> String uses {process} = match process.run("git", args: ["pull", "--ff-only"]) {
  Ok(o) => "{o.status}: {o.stdout}"
  Err(e) => "{e}"
}

test "runs are answered from recordings" {
  let ran = fakes(process, from: "recordings/")
  with ran
  expect(pull() == "0: Already up to date.")
  let deploy = match process.run("deploy", args: ["two words"]) {
    Ok(_) => "ran"
    Err(NotStarted(program, reason)) => "{program}: {reason}"
  }
  expect(deploy == "deploy: no such program")
  let other = match process.run("make", input: "x") {
    Ok(o) => o.status
    Err(_) => -1
  }
  expect(other == 3)
  let [first, second, third] = ran.calls(process.run) else { unreachable("not three runs") }
  expect(first.program == "git" && first.args == ["pull", "--ff-only"] && first.dir == None)
  expect(second.args == ["two words"])
  expect(third.program == "make" && third.input == "x")
}

test "a command no exchange answers fails the test" {
  with fakes(process, from: "strict/")
  let _ = process.run("rm", args: ["-rf", "/tmp/x"])
}
`

// TestProcessFakes answers process.run from recordings by command line,
// with a fallback for the rest, and lists the runs it answered.
func TestProcessFakes(t *testing.T) {
	prog := load(t, processProgram, map[string]string{
		"recordings/process.json": `{ "contract": "process", "exchanges": [
  { "op": "run", "key": "git pull --ff-only",
    "result": { "kind": "Ok", "value": { "status": 0, "stdout": "Already up to date.", "stderr": "" } } },
  { "op": "run", "key": "deploy \"two words\"",
    "result": { "kind": "Err", "value": { "kind": "NotStarted", "program": "deploy", "reason": "no such program" } } },
  { "op": "run",
    "result": { "kind": "Ok", "value": { "status": 3, "stdout": "", "stderr": "failed" } } }
] }`,
		"strict/process.json": `{ "contract": "process", "exchanges": [
  { "op": "run", "key": "ls", "result": { "kind": "Ok", "value": { "status": 0, "stdout": "", "stderr": "" } } }
] }`,
	})
	rs := RunTests(prog, "")
	if len(rs) != 2 {
		t.Fatalf("%d tests ran, want 2", len(rs))
	}
	if rs[0].Failure != "" {
		t.Errorf("%s: %s", rs[0].Name, rs[0].Failure)
	}
	want := `strict/process.json has no exchange for process.run of rm -rf /tmp/x: add one with "key": "rm -rf /tmp/x"`
	if !strings.Contains(rs[1].Failure, want) {
		t.Errorf("%s failed with %q, want %q", rs[1].Name, rs[1].Failure, want)
	}
}
