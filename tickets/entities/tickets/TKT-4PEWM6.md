---
id: TKT-4PEWM6
type: ticket
title: 'simpleMarkdownToHTML runs goldmark WithUnsafe(): safe only for operator-authored input'
kind: refactor
priority: medium
effort: m
status: backlog
---

## Description

Raised by the G705 (XSS) enablement work in TKT-R8MEE2, and deliberately left out
of that PR's scope.

`simpleMarkdownToHTML` runs goldmark with `html.WithUnsafe()`, so raw HTML in its
input passes through unsanitized. There is no HTML sanitizer anywhere in the repo.

Today this is safe, but only because of *who writes the input*, not because of
any escaping: the sole caller passes `metamodel.EntityDef.Description`, loaded by
`yaml.Unmarshal` from on-disk `metamodel.yaml`. That was verified by enumerating
writers — the only metamodel writers are CLI-only, none imported by
`dataentry`/`mcp`/`lua`/`rela-server`, and Lua's `rela.write_file` is confined to
`outputDir` by `filepath.IsLocal`.

The risk is that this is an *input-trust* invariant with nothing enforcing it.
Routing user-authored entity content through `simpleMarkdownToHTML` — which is a
plausible future change, since entity descriptions are user-writable elsewhere in
the product — would be a real stored-XSS bug, and nothing in the type system or
the tests would stop it. The constraint is currently documented at the call site
only.

TKT-R8MEE2 did not change the converter because doing so would alter rendering
behaviour well beyond that PR's scope.

## Solution

Options, in rough order of preference:

- Add an HTML sanitizer (e.g. bluemonday) on the output and drop `WithUnsafe()`,
accepting the rendering change for operator-authored markdown.
- Keep `WithUnsafe()` but make the trust boundary enforceable rather than
documented: a distinct input type (e.g. `OperatorMarkdown string`) that only
metamodel loading can produce, so passing request-derived content is a compile
error.
- At minimum, a test that pins the caller set, so a new call site fails loudly and
forces re-review — the same technique TKT-R8MEE2 used for the constant-slog-message
invariant in G706.
