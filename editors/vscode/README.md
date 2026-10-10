# Eyelang for VS Code

Highlighting for `.eye` files, and, through the language server `eye lsp`:

- errors as you type,
- formatting (`eye fmt`; on save by default),
- an outline of declarations, and folding,
- finer highlighting: contextual keywords (`type`, `state`, `view`, …) only where they are
  keywords, parameters, fields, variants, and **waiting calls** (`sleep!(…)`, `w.read!()`), which
  get the colour of control flow because a job may pause there.
- **go to definition** (F12, or Ctrl/Cmd-click) on any name: declarations in this module, in
  imported modules, in the prelude and the standard modules (`check/prelude.eye`, `check/std/`);
  locals, parameters, `self` and type variables; fields and methods (`x.name`, `x.send()`),
  record fields and named arguments, by the types of values. Built-ins go to the Go behind them:
  a function such as `print` or `now` to its implementation, a type such as `String` or `List`
  to where the checker makes it, an attribute such as `@on` to its declaration, and an `extern
  fn` whose `@host` the interpreter provides, such as `ai.generate`, to the Go host function.
- **find references** (Shift-F12, or Ctrl/Cmd-click on a declaration): every use in the program,
  across its modules. A name from the prelude, a standard module or the language itself is looked
  for in every program in the workspace (each directory with an `eye.toml`).

Fields and methods resolve once the program type-checks: while it has errors, go-to-definition
still finds every other name. Hover and completion are coming.

## Programs and eye.toml

Go-to-definition checks the module of the file you are in, with what it imports; find references
checks every module of the program. Imports are
directories below the program's root, and the root is the nearest directory above the file that
holds an `eye.toml` (it may be empty); without one, it is the file's own directory. So in this
repository `testdata/lab/eye.toml` makes `testdata/lab/digest/digest.eye` part of the lab program,
and its `import home` finds `testdata/lab/home`. `eye check file.eye` uses the same rule.

The prelude and standard modules live inside the `eye` binary. When the repository `eye` was built
from is on disk, definitions open its `check/` files; otherwise a read-only copy is written to
your cache directory. The Go behind built-ins opens only when its source is on disk: a binary
built with `-trimpath` finds it in an open workspace folder that holds the eyelang repository.

## Install

Download `eyelang.vsix` or build it from the repository root with `task vscode` (it lands in
`bin/eyelang.vsix`), then install it: **Extensions → ⋯ → Install from VSIX…**, or
`code --install-extension bin/eyelang.vsix`. Highlighting works straight away.

Errors, formatting, the outline and go-to-definition need the `eye` binary. The extension looks for it in
`eyelang.server.path` if you set it, then `bin/eye` in your workspace folders (what `task build`
writes in this repository), then your `PATH`, then `~/go/bin` (where `go install ./cmd/eye`
puts it). It checks every two seconds, so the server starts as soon as you build `eye`, and
restarts when you rebuild it. If the eyelang repository is open, the warning about a missing
binary offers to build it for you (it needs Go).

To work on the extension itself, open this directory in VS Code and press F5.

`.eye` files get the eye icon from `icons/eye-file.svg` in file icon themes that don't have one of
their own, such as the default Seti. The Marketplace needs the extension's icon as a PNG, so
`icons/eyelang.png` is rendered from `icons/eyelang.svg`:
`rsvg-convert -w 256 -h 256 icons/eyelang.svg -o icons/eyelang.png`.

## Without an editor

`eye check [dir]` reports a program's errors as `file:line:column: message` and exits 1 if there
are any: syntax errors, undefined names and misplaced waiting calls (types come with phase 2).
`eye check --syntax [paths]` checks only syntax, of any files, in parallel; it is what the editor
shows as you type, and takes a few milliseconds for a project.

## Tests

`npm test` checks the grammar's scopes against the assertions in `test/highlighting.eye`, that no
file in `testdata/syntax/valid` gets an `invalid` scope or ends inside a string, and that `eye lsp`
answers the JSON-RPC client the extension uses. `npm run test:vscode` runs the extension in a real
VS Code (downloaded once into `.vscode-test/`, or `VSCODE_EXECUTABLE`): it starts without a server,
starts one when `bin/eye` appears, restarts it when `bin/eye` changes, and checks errors,
formatting, the outline, semantic tokens and go-to-definition through it. On Linux without a
display, run it under `xvfb-run`.

It runs beside a VS Code you have open: the test copy gets a short user data directory of its own,
and a `PATH` without any directory that holds an `eye` (such as `scripts/`, which direnv adds).

From the repository root, `task vscode:unit`, `task vscode:e2e` and `task vscode:package` run one
step each, and `task vscode` runs all three. They run `npm ci` only when `package-lock.json` has
changed since the last install.
