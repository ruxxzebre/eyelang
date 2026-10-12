# SPECS.md section 20, example 2.
import process

fn pull() -> Int uses {process} = match process.run("git", args: ["pull", "--ff-only"]) {
  Ok(output) => output.status
  Err(_) => -1
}

test "pull runs git once" {
  let ran = fakes(process, from: "recordings/")
  with ran
  expect(pull() == 0)
  let [git] = ran.calls(process.run) else { unreachable("not one run") }
  expect(git.args == ["pull", "--ff-only"])
}
