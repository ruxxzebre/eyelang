## Running other programs.
##
## A program runs without a shell, in the directory the program was started
## in, or in a test, in the test's own empty directory. In a job, each run
## is a step: `process.run!("git", args: ["pull"])`. A test answers runs from
## recordings with `with fakes(process, from: "recordings/")`, and lists them
## with `calls(process.run)`.

# SPECS.md §12. Its primitives are provided by the host.

## What a program did: its exit status, and what it wrote to its standard
## output and standard error, as text.
pub type Output = { status: Int, stdout: String, stderr: String }

## Why a program didn't run.
pub type ProcessError =
  ## It couldn't be started: there is no such program, or it can't be run.
  | NotStarted(program: String, reason: String)

## Runs a program with its arguments and waits for it to finish. input goes
## to its standard input, and dir is where it runs. Any exit status is Ok, as
## any status of an HTTP answer is: check `output.status == 0`. Err means it
## didn't start.
@host("process.run")
pub extern fn run(
  program: String,
  args: List(String) = [],
  input: String = "",
  dir: Option(String) = None,
) -> Result(Output, ProcessError) uses {process}
