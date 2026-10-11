## Files and directories on the machine the program runs on.
##
## A relative path is relative to the directory the program was started in;
## in a test, to an empty directory of the test's own, removed when the test
## ends. Text is UTF-8. In a job, each call is a step: `fs.readText!(path)`.

# SPECS.md §12. Its primitives are provided by the host.

## Why a file or directory couldn't be read or changed.
pub type FsError =
  ## Nothing is at the path.
  | NotFound(path: String)
  ## The program may not read or change what is at the path.
  | NotAllowed(path: String)
  ## Something is at the path already.
  | AlreadyExists(path: String)
  ## Any other failure, such as a file that isn't UTF-8 text, as the system
  ## describes it.
  | SystemError(path: String, reason: String)

impl Show for FsError {
  fn show(self) -> String = match self {
    NotFound(path) => "{path}: not found"
    NotAllowed(path) => "{path}: not allowed"
    AlreadyExists(path) => "{path}: already exists"
    SystemError(path, reason) => "{path}: {reason}"
  }
}

## The text of a file.
@host("fs.readText")
pub extern fn readText(path: String) -> Result(String, FsError) uses {fs}

## Writes text to a file, which it creates or replaces.
@host("fs.writeText")
pub extern fn writeText(path: String, text: String) -> Result((), FsError) uses {fs}

## Adds text to the end of a file, which it creates if there is none.
@host("fs.appendText")
pub extern fn appendText(path: String, text: String) -> Result((), FsError) uses {fs}

## Whether a file or directory is at the path.
@host("fs.exists")
pub extern fn exists(path: String) -> Bool uses {fs}

## The names of what a directory holds, in order.
@host("fs.list")
pub extern fn list(dir: String) -> Result(List(String), FsError) uses {fs}

## Makes a directory, and the directories above it that are missing. A
## directory that is there already is Ok.
@host("fs.makeDir")
pub extern fn makeDir(path: String) -> Result((), FsError) uses {fs}

## Removes a file or an empty directory.
@host("fs.remove")
pub extern fn remove(path: String) -> Result((), FsError) uses {fs}
