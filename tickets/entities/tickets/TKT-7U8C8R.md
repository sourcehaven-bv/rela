---
id: TKT-7U8C8R
type: ticket
title: 'Enable gosec G706 (log injection), scope-exclude dataentry with a tested invariant'
kind: refactor
priority: medium
effort: s
status: done
---

## Description

`G706` was in the `gosec.excludes` block in `.golangci.yml`, so log-injection
taint analysis never ran. Enabling it repo-wide surfaced 20 findings, all in
`internal/dataentry` and zero elsewhere.

Every finding is the identical shape:

```go
slog.Warn("dataentry: read attachment failed", "err", err, "entity", entityID)
```

A constant message literal, with user data only ever as a structured attribute.
No site in the package uses `fmt.Sprintf` into the message or a `slog.*f`
variant.

Why they are false positives was established from gosec's own source rather than
inferred: `analyzers/loginjection.go` registers `log/slog.{Info,Warn,Error,Debug}`
as sinks and flags *any* tainted value reaching one — it never distinguishes
message from attribute, and cannot follow the value into the handler. Tellingly,
`strconv.Quote` is on gosec's own sanitizer list and `TextHandler` uses exactly
that internally; gosec simply can't see through the indirection. Every entry point
installs `slog.NewTextHandler`, verified empirically: an injected
`"bob\nlevel=ERROR msg=\"forged\""` comes out escaped on a single line.

## Solution

A narrow path-scoped exclusion for `internal/dataentry` rather than 20 `//nosec`
comments. The codebase has **zero** prior `//nosec` uses, so adding 20 for a
provable non-issue would introduce a new pattern at scale. Instead the rationale
lives in one place, and `internal/dataentry/loginjection_test.go` pins the two
properties the safety actually rests on:

- `TestSlogMessagesAreConstant` — AST walk asserting every slog message in the
package is a compile-time constant.
- `TestSlogTextHandlerEscapesNewlines` — pins the handler's escaping guarantee.

This matters because `sloglint` at default settings does **not** forbid a dynamic
message, so the first property was previously unenforced convention. The
exclusion is only sound because the test now enforces it.
