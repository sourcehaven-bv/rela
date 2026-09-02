---
id: RR-KA9LD5
type: review-response
title: Two comments in cascadehost.go stated the pre-fix behaviour as fact
finding: One cited the very limitation this ticket removed and pointed at the function that no longer has it
severity: significant
resolution: Both comments rewritten. recordCascade's doc no longer cites the removed limitation and now explains that its guard is what makes the runner's per-entry label survive; the cascadeHost type doc now describes the real label set.
status: addressed
---

`cascadehost.go`'s `recordCascade` doc read: *"the generic label (per runner.go
applyRelationCreations rationale — automation.Result doesn't carry per-action
names through the engine)"* — a direct citation of the limitation this ticket
removes, pointing at the function whose new doc says the opposite.

The `cascadeHost` type doc read: *"Records carry triggered_by=\"automation\" (or
the cascade-delete label when invoked from IfExistsReplace)"* — now wrong in the
common case.

Both rewritten. `recordCascade`'s doc now also explains that its `if` is what
makes the runner's label survive, and cross-references the mirrored guard in
`triggeredByCtx`.
