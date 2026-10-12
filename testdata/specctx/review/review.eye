# What SPECS.md §19's agent sessions assume (context=review): GitHub's pull
# requests, a checkout of one, and the tools an agent may use on it. The
# agents module itself is ROADMAP task 6.8.

import fs
import github.{PullRequest}

## Checks out a commit into a directory of its own, and returns its path.
fn checkout(commit: String) -> String uses {fs} {
  let dir = "work/{commit}"
  let _ = fs.makeDir(dir)
  dir
}

## Reads a file of the checkout.
fn readFiles(path: String) -> String uses {fs} = fs.readText(path) ?? ""

## Writes a file of the checkout.
fn editFiles(path: String, text: String) -> Bool uses {fs} = fs.writeText(path, text) == Ok(())

## Runs the tests of the checkout in dir, and says whether they passed.
fn runTests(dir: String) -> Bool uses {fs} = fs.exists("{dir}/PASSED")
