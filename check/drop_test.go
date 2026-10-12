package check

import (
	"slices"
	"strings"
	"testing"
)

// TestDroppedResults checks that a statement can't drop a Result or a
// command's outcome without a word (SPECS.md §10 and §17): each must be
// used, matched, passed on with ?, or discarded with `let _ = …`. The value
// of a block's last expression is used when the block's is, and other
// values, () among them, may be dropped. Each case lists every diagnostic
// the program has, exactly, as "file:line:column: message".
func TestDroppedResults(t *testing.T) {
	const result = "so a failure would go unnoticed: use it, match it, pass its error on with ?, or discard it with let _ = …"
	const outcome = "so a refusal would go unnoticed: match it, or discard it with let _ = …"
	others := map[string]string{"svc/svc.eye": contract, "shelves/shelves.eye": shelves}
	cases := []struct {
		name string
		src  string
		want []string
	}{
		{"a Result as a statement", `import fs
pub fn main() -> Result((), fs.FsError) uses {fs} {
  fs.writeText("a", "b")
  Ok(())
}`, []string{
			"main.eye:3:3: this drops a Result((), FsError), " + result,
		}},
		{"a Result as the last expression of a function that returns nothing", `fn check(s: String) { parseInt(s) }`, []string{
			"main.eye:1:23: this drops a Result(Int, String), " + result,
		}},
		{"a Result in a branch, a loop and a match arm", `fn each(xs: List(String), quiet: Bool) {
  if !quiet { parseInt("1") }
  for x in xs { parseInt(x) }
  match xs {
    [] => ()
    _ => parseInt("2")
  }
}`, []string{
			"main.eye:2:15: this drops a Result(Int, String), " + result,
			"main.eye:3:17: this drops a Result(Int, String), " + result,
			"main.eye:6:10: this drops a Result(Int, String), " + result,
		}},
		{"a command's outcome, which may be Uncertain", `import svc
job send(text: String) {
  svc.send!(text, key: "k-{text}")
  svc.raw!(text)
}`, []string{
			"main.eye:3:3: this drops Sent, a command's outcome, which may be Uncertain, " + outcome,
			"main.eye:4:3: this drops Raw, a command's outcome, which may be Uncertain, " + outcome,
		}},
		{"a reconciled outcome is still an outcome", `import svc
job send(text: String) {
  svc.send!(text, key: "k-{text}").reconcile!()
}`, []string{
			"main.eye:3:3: this drops Sent, a command's outcome, " + outcome,
		}},
		{"a @fresh command's outcome may be Stale or Expired", `import shelves
job tidy(id: String) {
  let shelf = shelves.shelf!(id)
  shelves.clear!(id, basedOn: shelf.version, key: "tidy-{id}")
}`, []string{
			"main.eye:4:3: this drops Cleared, a command's outcome, which may be Uncertain, Stale or Expired, " + outcome,
		}},
		{"what is handled, discarded or not a Result", `import fs
import svc
fn read(s: String) -> Result(Int, String) {
  let _ = parseInt("x")
  let n = parseInt(s)?
  match parseInt(s) {
    Ok(m) => print("{m}")
    Err(e) => log.warn(e)
  }
  [1, 2] |> first
  print("{n}")
  let checked = if s == "" {
    Ok(0)
  } else {
    let t = s |> trim
    parseInt(t)
  }
  checked
}
fn noted(path: String) uses {fs} = if fs.exists(path) then fs.readText(path) else Ok("")
job send(text: String) {
  let _ = svc.send!(text, key: "k-{text}")
  let r = svc.send!(text, key: "r-{text}").reconcile!()
  match svc.raw!(text) {
    Done | Uncertain(_) => ()
  }
}`, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := diagnoseWith(t, c.src, others)
			if !slices.Equal(got, c.want) {
				t.Errorf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(c.want, "\n"))
			}
		})
	}
}
