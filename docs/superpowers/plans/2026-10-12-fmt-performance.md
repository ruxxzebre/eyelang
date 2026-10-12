# Formatter Performance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce directory discovery, program checking and formatting time while preserving syntax diagnostics and inferred public effects.

**Architecture:** Recursive discovery skips hidden directories and node_modules, but accepts either when explicitly named. A formatting session shares only immutable parsed files across otherwise independent program checks and omits the name index. Formatting accepts a parsed syntax file, reusing the checker's tree only when its bytes still match the current source.

**Tech Stack:** Go, the lossless syntax parser, existing checker and formatter tests.

Execution is inline in the already authorized session. Do not touch the separately claimed check/index.go. Do not batch unrelated modules into one check: an error in one must not suppress another module's valid effects. Keep rereading disk sources so the syntax cache cannot hide a changed file.

## Task 1: Discovery (completed)

Files: cmd/eye/fmt.go; cmd/eye/fmt_test.go; docs/reference/status.md.

- [ ] Add a discovery test with main.eye, sub/main.eye, .claude/worktrees/old/main.eye, sub/.eye/main.eye and sub/node_modules/pkg/main.eye. Only the first two are discovered from the root. Explicit .claude and node_modules roots must discover their contents. Use the existing program(t, files) fixture writer.
- [ ] In the WalkDir callback, after checking err, add:

```go
if path != p && d.IsDir() && (strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules") {
    return filepath.SkipDir
}
```

- [ ] Explain these skips and explicit-root behavior in CLI help and the tooling status row. Run `go test ./cmd/eye -run 'TestEyeFiles|TestCheckSyntax|TestFmt' -count=1`.

## Task 2: Reuse parsed syntax when formatting (completed)

Files: syntax/format.go; syntax/format_test.go; cmd/eye/fmt.go.

- [ ] Make Format delegate to FormatFile(Parse(src)); add the following entry point:

```go
func FormatFile(f *File) ([]byte, error) {
    if len(f.Diagnostics) > 0 {
        return nil, errors.New("the file has syntax errors")
    }
    fm := &formatter{src: f.Src}
    fm.collect(f.Root, nil)
    fm.plan(f.Root)
    return fm.print(), nil
}
```

- [ ] Split the CLI helper: format calls formatParsed(path, syntax.Parse(src), stderr); formatParsed reports the existing positional diagnostics and then calls syntax.FormatFile(f).
- [ ] Extend TestFormatKeepsMeaning to reuse its original parsed file through FormatFile, checking output equivalence, unchanged original tree and repeatability. Keep Format's error-refusal test and add the same assertion for FormatFile(Parse(invalid)). Run `go test ./syntax -run TestFormat -count=1`.

## Task 3: Reuse syntax across program checks, omit unused indexing (completed)

Files: check/format.go; check/format_test.go; check/check.go; check/builtindecls.go; cmd/eye/fmt.go.

- [ ] Add a zero-value usable FormatSession containing `files map[string]*ir.File`. Its Check(dir, module) resolves module relative to dir as existing Options.Module does, creates a fresh checker with `parsed: s.files` and no index, calls c.program(path), and returns Result with the program, diagnostics and checker. Its File(path) returns the stored file's syntax tree or nil.
- [ ] Add `parsed map[string]*ir.File` to checker and this helper in check/format.go:

```go
func (c *checker) parseFile(path string, src []byte) *ir.File {
    if f := c.parsed[path]; f != nil && bytes.Equal(f.Src, src) {
        return f
    }
    f := parseFile(path, src)
    if c.parsed != nil {
        c.parsed[path] = f
    }
    return f
}
```

- [ ] Use c.parseFile in program's prelude/store setup, disk/overlay module loading, standard-library imports and loadBuiltins. Leave ordinary Check and Program behavior intact when no session is provided. Cache syntax only; never cache mutable inferred types or IR.
- [ ] Create one session in runFmt, pass it into signatures and use session.Check instead of check.Check. For each source after writeUses, reuse session.File(absPath(path)) only if bytes.Equal(in, f.Src); otherwise parse in anew and pass it to formatParsed.
- [ ] Test two independent roots importing the same module: each result has fresh IR, identical effects and diagnostics to Check, shared immutable syntax and no index. Modify an imported source between checks and verify the next check sees the new effects and a different tree. Test syntax/type failures retain diagnostics and make Signatures return false. Run `go test ./check ./cmd/eye ./syntax ./internal/lsp -count=1` and targeted race tests.

## Task 4: Verify and measure (completed)

- [ ] Save a baseline CLI binary before implementation. Compare its output against the new binary on the same explicitly selected nonhidden directories, checking stdout, stderr and exit code.
- [ ] Time both binaries on the repository root and on a clean workload. Use repeated subprocess runs with captured output and report medians. Separately time eyeFiles, signatures and the formatting loop to compare the three stages with the earlier profile.
- [ ] Run go vet for the touched packages and the required full repository check before any push. Record unrelated baseline failures separately; do not modify unrelated claims or fixtures.
- [ ] Verify git diff --check and leave the finished change reviewable in the shared checkout.

## Task 5: Remove the wrapper's warm build overhead (completed)

Files: scripts/eye; cmd/eye/script_test.go.

The user's shell timing still measured 0.613 seconds after the formatter changes; direct bin/eye took 0.158 seconds. The wrapper's unconditional go build is the remaining cost.

- [ ] Before editing the wrapper, record these paired timings. Add wrapper tests using a fake Go executable: unchanged inputs build once; source edits, file deletion and build-setting changes rebuild; a failed rebuild does not execute the old binary; workspace modules and replace directives always use go build.
- [ ] Cache the effective `go env -json` and VCS state in bin/.eye-build-key. Save a pre-build timestamp in bin/.eye-build-stamp, so changes made during a build are noticed on the next invocation. Search for files or directories newer than the timestamp while pruning hidden directories, node_modules and bin. Directory timestamps detect deleted files. Explicitly retain the unconditional build for workspace modules and replace directives, whose external files cannot be checked by the repository scan.
- [ ] Use temporary stamp/key files and rename them only after go build succeeds; remove temporary files on failure. Keep all caches under the existing ignored bin directory. Run wrapper tests, shell syntax checks, then measure the wrapper's warm time again.

This fast path is for ordinary edits in this checkout; intentionally backdating source timestamps requires a rebuild by removing bin/.eye-build-stamp or running task build.

## Verification results

- Discovery, check-session freshness/IR isolation, and formatter tree-preservation tests pass. Existing effects-writing, syntax, CLI and LSP tests pass. Targeted race tests pass.
- CLI stdout, stderr and exit status match the baseline for the same visible files and for an explicitly selected hidden worktree directory.
- With default recursive exclusions, 334 files replace the accidental 2,895-file workload. Warm stage measurements: discovery 5.4 ms, program checks 121 ms, formatting 22.8 ms. The corresponding original stage measurements were 369 ms, 2,577 ms and 502 ms.
- The same visible-file workload takes about 0.16 seconds instead of 0.39 seconds; syntax-only checking about 0.054 seconds instead of 0.136 seconds.
- Wrapper rebuild tests cover edits, additions, deletions, build flags, failed builds and external-module fallbacks. Warm wrapped checks take about 0.25 seconds, compared with the user's reproduced 0.613 seconds. Direct checks take about 0.15 seconds. Rebuilds after changes remain intentional.
- Full task check passed generated-file verification, build, vet, lint, all Go tests, formatting, conformance, grammar, extension unit tests/packaging and the docs build. It failed only at the final pre-existing roadmap check: What's next names closed issue #83. The extension's GUI end-to-end test is skipped by the existing macOS task.
- The final full check, including the wrapper tests, passed those same code checks. Its roadmap check also found G.20's issue #119 had closed while the row still lacked `(done)`. These unrelated roadmap entries were left unchanged.
- Rebased onto the concurrent language/tooling commits, preserving variant-choice state and hexadecimal-literal normalization in the two conflicts. Full task check again passed every code check; only the existing closed-issue #83 roadmap reference remains. The upstream commits corrected the #119 bookkeeping.
