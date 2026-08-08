---
id: RR-Q1VCKR
type: review-response
title: Stale Redacted survives the nothing-hidden fast path if an entity is re-redacted
finding: 'visibility.Redact''s len(hidden)==0 branch returns the input pointer untouched (the documented allocation-free identity guarantee). An entity that ALREADY carries Redacted from a prior pass therefore keeps it even though this redactor hid nothing — a silent lie. Reviewer verified: ''Redacted after nothing-hidden pass: [salary]''. Not reachable today (every Redact caller was checked: views.go:65 runs once on a freshly-loaded entry, export.go:311 and export_list.go:447 redact store-fresh neighbors, traversal reloads from the store), but it becomes wrong the moment anyone stacks two readers or re-redacts a passed-through entity.'
severity: minor
resolution: 'Documented as a precondition on Redact''s godoc rather than fixed in code: clearing the field unconditionally would allocate on the no-policy path and give up the byte-identical/allocation-free guarantee that the fast path exists for. The godoc now states that e must be a raw store entity and never the output of a prior Redact, and notes no production caller stacks readers today. A code fix would be the right call if reader stacking is ever introduced.'
status: addressed
---

## Finding (from cranky-code-reviewer)

`internal/visibility/policyreader.go` — the fast path:

```go
hidden := red.HiddenProperties(ctx, e)
if len(hidden) == 0 {
    return e   // untouched, including any pre-existing Redacted
}
```

Feed `Redact` an entity that a previous redactor already marked, with a redactor
that hides nothing, and the stale marker survives as if this redactor had
produced it.

## Why documented rather than fixed

Clearing `Redacted` on the fast path means allocating a copy on the no-policy
path, which is precisely what that branch exists to avoid — the godoc promises
byte-identical, allocation-free behavior under `NopRedactor`. Paying that on
every read to defend against a caller pattern nobody uses is the wrong trade.

The reviewer independently confirmed no production caller stacks readers:
`views.go:65` runs once on a freshly-loaded entry (and the `"entry"` collection
is deleted beforehand), `export.go:311` and `export_list.go:447` redact
store-fresh neighbors, and traversal reloads from the store.

So the honest fix is a stated precondition, now on `Redact`'s godoc:

> PRECONDITION: e must be a raw store entity, never the output of a
> prior Redact. On the nothing-hidden path the input is returned
> untouched, so a stale Redacted from an earlier pass would survive and
> misreport as this redactor's verdict.

## Revisit if

Reader stacking or a re-redaction path is ever introduced. At that point the
precondition stops being enforceable by convention and should become code —
likely by clearing on the copy path and accepting the allocation.
