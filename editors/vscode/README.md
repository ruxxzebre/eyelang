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

From the repository root:

```sh
task vscode       # builds bin/eye and bin/eyelang.vsix
code --install-extension bin/eyelang.vsix
```

The extension runs `eye lsp`, so put `bin/eye` on your `PATH` or set **Eyelang › Server: Path**
(`eyelang.server.path`) to it. Without the server you still get highlighting. After rebuilding
`eye`, run **Eyelang: Restart Language Server**.

To work on the extension itself, open this directory in VS Code and press F5.

## Without an editor

`eye check [paths]` reports errors as `file:line:column: message`, the same ones the editor
shows, and exits 1 if there are any. It parses files in parallel and takes a few milliseconds for
a project. Until the checker lands it reports syntax errors only.

## Tests

`npm test` checks the grammar's scopes against the assertions in `test/highlighting.eye`, and
that no file in `testdata/syntax/valid` gets an `invalid` scope or ends inside a string.
