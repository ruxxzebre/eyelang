# tree-sitter-eyelang

The [tree-sitter](https://tree-sitter.github.io) grammar for Eyelang 2. It is the definition of the
language's syntax: [SPECS.md](../SPECS.md) section 23 summarises it, and the compiler's hand-written
parser in [`syntax/`](../syntax) must agree with it on every file in
[`testdata/syntax`](../testdata/syntax).

| Path | Contents |
|---|---|
| `grammar.js` | The grammar |
| `src/scanner.c` | The external scanner: statement-ending newlines, a block's `{`, string text |
| `test/corpus/` | Corpus tests: inputs and the trees they must parse to |
| `queries/highlights.scm` | Highlighting, with nvim-treesitter's capture names |
| `queries/locals.scm` | Scopes and definitions |
| `queries/indents.scm` | Indentation that matches `eye fmt` |

The generated parser (`src/parser.c` and friends) isn't committed. Generate it with
`tree-sitter generate` (tree-sitter-cli 0.27), or run `task generate` from the repository root.

## Working on the grammar

```sh
tree-sitter generate        # after editing grammar.js
tree-sitter test            # the corpus
tree-sitter parse file.eye  # print a tree
```

A syntax change also needs the Go parser and the conformance suite; see
[CLAUDE.md](../CLAUDE.md#how-to-make-a-change). `task check` at the repository root runs everything.

## Editors

Waiting calls (`sleep!(…)`, `w.read!()`) are highlighted as `@keyword.coroutine`, the group
colour schemes use for `await`, so every point where a job can pause stands out.

### Neovim

Neovim 0.10 or later highlights with tree-sitter on its own. Build the parser into your runtime
path and copy the queries next to it:

```sh
cd tree-sitter-eyelang
tree-sitter generate
tree-sitter build -o ~/.local/share/nvim/site/parser/eyelang.so
mkdir -p ~/.local/share/nvim/site/queries/eyelang
cp queries/*.scm ~/.local/share/nvim/site/queries/eyelang/
```

Then, in `init.lua`:

```lua
vim.filetype.add({ extension = { eye = "eyelang" } })
vim.api.nvim_create_autocmd("FileType", {
  pattern = "eyelang",
  callback = function() vim.treesitter.start() end,
})
```

With nvim-treesitter, register the parser with its custom-parser mechanism instead, pointing at
this directory, and keep the queries in `queries/eyelang/` on your runtime path.

To format with `eye fmt`, which reads standard input when given no files:

```lua
vim.api.nvim_create_autocmd("FileType", {
  pattern = "eyelang",
  callback = function() vim.bo.formatprg = "eye fmt" end,
})
```

### Helix

Add the language and grammar to `~/.config/helix/languages.toml`:

```toml
[[language]]
name = "eyelang"
scope = "source.eyelang"
file-types = ["eye"]
comment-token = "#"
indent = { tab-width = 2, unit = "  " }
formatter = { command = "eye", args = ["fmt"] }

[[grammar]]
name = "eyelang"
source = { path = "/path/to/eyelang/tree-sitter-eyelang" }
```

Then run `hx --grammar build` and copy `queries/*.scm` to
`~/.config/helix/runtime/queries/eyelang/`. Helix falls back from a capture name it doesn't know
to its parent (`keyword.coroutine` to `keyword`), so the queries work unchanged.

### VS Code

VS Code doesn't use tree-sitter. Its extension, with highlighting and the `eye lsp` language
server, is in [`editors/vscode`](../editors/vscode).
