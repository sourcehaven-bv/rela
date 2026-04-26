---
id: TKT-KXLWA
type: ticket
title: Surface Lua errors from validation rules
kind: enhancement
priority: medium
effort: m
status: in-progress
---

Validation rules in `internal/validation/lua.go` currently swallow Lua compile
and runtime errors via `slog.Warn` and fail open (return no violations).
Operators running `rela analyze` see no diagnostic about why a rule was skipped
— only the absence of expected violations.

Follow-up to TKT-LR5YC: extend the `*lua.ScriptError` envelope (with `Surface =
"validation"`) to validation rule execution, so `rela analyze` can surface
structured Lua errors (path, line, message, source slice, stack) to the CLI
rather than burying them in slog.

Note: `validateLua` calls `ls.PCall` directly rather than going through
`Runtime.RunString`/`RunFileContent`, so it bypasses the message handler that
captures stack frames. Adopting `*ScriptError` here will require either routing
through Runtime or extracting the stack-capture helper.

Fail-open semantics should be preserved (one bad rule must not abort the entire
analyze run) — but the error becomes visible rather than invisible.
