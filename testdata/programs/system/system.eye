# Files, programs and random numbers: the standard modules fs, process and
# random (SPECS.md §12). Each test has a directory of its own, and its random
# numbers come from the same seed every time.

import fs.{FsError}
import process
import random

type CountError =
  | Unreadable(FsError)
  | BadNumber(String)

## The number in a file, as SPECS.md §10's example reads it.
pub fn count(path: String) -> Result(Int, CountError) uses {fs} {
  let text = fs.readText(path)?
  let n = parseInt(text).mapErr(BadNumber)?
  Ok(n)
}

## The same, for a program that only reports what went wrong.
pub fn total(paths: List(String)) -> Result(Int, Problem) uses {fs} {
  var sum = 0
  for path in paths {
    sum += fs.readText(path).context("reading {path}")? |> parseInt?
  }
  Ok(sum)
}

test "files are written, read, listed and removed" {
  expect(fs.writeText("a.txt", "4") == Ok(()))
  expect(fs.appendText("a.txt", "2") == Ok(()))
  expect(fs.readText("a.txt") == Ok("42"))
  expect(count("a.txt") == Ok(42))
  expect(fs.makeDir("notes/old") == Ok(()))
  expect(fs.writeText("notes/b.txt", "x") == Ok(()))
  expect(fs.list("notes") == Ok(["b.txt", "old"]))
  expect(fs.exists("notes/b.txt"))
  expect(fs.remove("notes/b.txt") == Ok(()))
  expect(!fs.exists("notes/b.txt"))
}

test "a test starts in an empty directory" {
  expect(fs.list(".") == Ok([]))
  expect(fs.readText("a.txt") == Err(NotFound(path: "a.txt")))
}

test "file errors are values, and show as what went wrong" {
  expect(count("missing.txt") == Err(Unreadable(NotFound(path: "missing.txt"))))
  let Err(p) = total(["missing.txt"]) else { return unreachable("there is no file") }
  expect("{p}" == "reading missing.txt: missing.txt: not found")
  expect(fs.makeDir("d") == Ok(()))
  expect(fs.writeText("d", "x") |> isErr)
}

test "programs run with arguments and input, and any status is Ok" {
  let Ok(out) = process.run("sh", args: ["-c", "cat; echo oops >&2; exit 3"], input: "hello") else {
    return unreachable("sh runs")
  }
  expect(out.status == 3)
  expect(out.stdout == "hello")
  expect(out.stderr == "oops\n")
  let Ok(pwd) = process.run("sh", args: ["-c", "ls"]) else { return unreachable("sh runs") }
  expect(pwd.stdout == "")
  expect(process.run("no-such-program-eye") |> isErr)
}

test "random numbers stay in their range, and repeat from test to test" {
  var rolls: List(Int) = []
  for _ in 0..200 {
    rolls += [random.int(1..=6) ?? 0]
  }
  expect(rolls |> all(fn r: r >= 1 && r <= 6))
  expect(rolls |> toSet |> size == 6)
  expect(rolls |> take(5) == [5, 4, 5, 5, 2])  # the seed is the same in every test
  expect(random.int(3..3) == None)
  let f = random.float()
  expect(f >= 0.0 && f < 1.0)
  expect(random.pick([7]) == Some(7))
  let none: List(Int) = []
  expect(random.pick(none) == None)
  expect(random.shuffle([1, 2, 3]) |> sort == [1, 2, 3])
}

## Copies a file an hour after it is asked to: each read and write is a step.
job copy(source: String, target: String) -> Result((), FsError) {
  let text = fs.readText!(source)?
  let n = random.int!(1..=6) ?? 0
  sleep!(1 h)
  fs.writeText!(target, "{text} {n}")
}

test "in a job, files and random numbers are steps" {
  expect(fs.writeText("in.txt", "x") == Ok(()))
  copy.start("in.txt", "out.txt")
  expect(!fs.exists("out.txt"))
  advance(1 h)
  expect(fs.readText("out.txt") == Ok("x 5"))
}
