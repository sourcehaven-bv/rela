---
id: TKT-NCLA67
type: ticket
title: Make searchVisibleHits fail closed when the searcher cannot redact fields
kind: enhancement
priority: medium
effort: s
status: done
---

## Description

`searchVisibleHits` (`internal/dataentry/helpers.go`) degrades silently when the
wired `visibleSearcher` does not satisfy `search.FieldVisibleSearcher`:

```go
fvs, ok := vs.(search.FieldVisibleSearcher)
if !ok || !aff.hidesAnyField() {
    return vs.SearchVisible(ctx, q, scope)   // no field redaction, no error
}
```

A future decorator — a cache, a metrics wrapper, a tracing shim — that wraps the
searcher without forwarding `SearchVisibleFields` makes the type assertion miss,
and search silently stops redacting fields **even when the active ACL policy
hides them**. No error, no log.

GitHub issue #1093 (IB-review rela#1089, finding 1). Severity: low, because the
`visible:` affordances are not yet in production.

## The precedent this misses

The same failure mode was identified and made fail-closed **one layer down**, in
`search.Visible.SearchVisibleFields` (RR-8W40EW). Its godoc is explicit:

> If a hidden func is supplied but this Visible has no provenance source ... the
> method FAILS CLOSED — it yields ErrScope rather than silently returning
> un-redacted hits. A missing provenance source is a wiring bug ... and silently
> skipping redaction is exactly the oracle this closes.

So the principle is already settled in this codebase; it simply was not carried
to the outer seam. The two decision points are the same shape and should behave
the same way.

## Note on the second condition

`!aff.hidesAnyField()` is a **different** case and must keep falling through: if
the policy hides nothing, there is nothing to redact and plain `SearchVisible`
is correct. Only the `!ok` half is a wiring bug.
