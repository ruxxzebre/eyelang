# Eyelang for VS Code

Highlighting for `.eye` files, and, through the language server `eye lsp`:

- errors as you type,
- formatting (`eye fmt`; on save by default),
- an outline of declarations, and folding,
- finer highlighting: contextual keywords (`type`, `state`, `view`, …) only where they are
  keywords, parameters, fields, variants, and **waiting calls** (`sleep!(…)`, `w.read!()`), which
  get the colour of control flow because a job may pause there.

Name resolution (hover, go to definition, completion) comes with the checker in phase 2.

## Install

Download `eyelang.vsix` or build it from the repository root with `task vscode` (it lands in
`bin/eyelang.vsix`), then install it: **Extensions → ⋯ → Install from VSIX…**, or
`code --install-extension bin/eyelang.vsix`. Highlighting works straight away.

Errors, formatting and the outline need the `eye` binary. The extension looks for it in
`eyelang.server.path` if you set it, then `bin/eye` in your workspace folders (what `task build`
writes in this repository), then your `PATH`, then `~/go/bin` (where `go install ./cmd/eye`
puts it). It checks every two seconds, so the server starts as soon as you build `eye`, and
restarts when you rebuild it. If the eyelang repository is open, the warning about a missing
binary offers to build it for you (it needs Go).

To work on the extension itself, open this directory in VS Code and press F5.

## Without an editor

`eye check [paths]` reports errors as `file:line:column: message`, the same ones the editor
shows, and exits 1 if there are any. It parses files in parallel and takes a few milliseconds for
a project. Until the checker lands it reports syntax errors only.

## Tests

`npm test` checks the grammar's scopes against the assertions in `test/highlighting.eye`, that no
file in `testdata/syntax/valid` gets an `invalid` scope or ends inside a string, and that `eye lsp`
answers the JSON-RPC client the extension uses. `npm run test:vscode` runs the extension in a real
VS Code (downloaded once into `.vscode-test/`, or `VSCODE_EXECUTABLE`): it starts without a server,
starts one when `bin/eye` appears, restarts it when `bin/eye` changes, and checks errors,
formatting, the outline and semantic tokens through it. On Linux without a display, run it under
`xvfb-run`.
