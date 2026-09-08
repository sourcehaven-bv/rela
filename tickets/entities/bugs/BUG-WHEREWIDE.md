---
id: BUG-WHEREWIDE
type: bug
title: 'An unparseable view where: silently widens the collection instead of failing'
severity: medium
status: backlog
description: 'A view `where:` clause that fails to parse is discarded, and the traversal continues with the UNFILTERED result set. A construct whose only job is to NARROW a collection fails by WIDENING it, silently. It should be a load error instead.'
---

**A `where:` clause that fails to parse is discarded, and the traversal
continues with the UNFILTERED result set.** A construct whose only job is to
narrow a collection fails open.

`internal/dataentry/views.go:106-112`:

```go
if rule.Where != "" {
    filtered, err := h.filterEntities(found, rule.Where)
    if err == nil {
        found = filtered
    }
    // On error, continue with unfiltered results (silent failure for robustness)
}
```

`filterEntities` (`views.go:194-198`) returns an error when `filter.Parse`
rejects the expression. The caller drops it on the floor.

## Why this is the wrong direction

The project already settled this argument for automations. From the root
`CLAUDE.md`:

> A `condition:` that fails to compile is a **load error**
> (`NewEngineFromMetamodel` returns one), as is an unparseable `when:` clause:
> **dropping a constraint widens the automation, so failing the load is the
> safe direction.**

The same reasoning applies verbatim to a view's `where:`. The comment calls
the current behaviour "robustness", but robustness here means showing the
operator MORE entities than the view asked for, silently, with no signal in
the response and (worse) no signal at load time.

## Reproduction — the error path is reachable

`filter.Parse` genuinely rejects malformed input (measured at `e717f1cc`, not
inferred):

```
"status=done"  err=<nil>                                          → parses
"(((("         err=invalid filter expression (missing operator)   → REJECTED
"status"       err=invalid filter expression (missing operator)   → REJECTED
""             err=empty filter expression                        → REJECTED
```

So a typo'd `where:` — e.g. `where: status` instead of `where: status=done` —
is an ordinary operator mistake, not a contrived one. It silently produces a
view containing every traversed entity.

Note `filter.Parse` is permissive in other ways (`a=b=c` and `>=` both parse
into something surprising), so this is specifically about the *rejected* set.

## Not a confidentiality bug

Collections ARE row-gated and field-redacted (`views.go:66-68`, via
`h.viewReader.Filter`) — a widened `where:` cannot leak an entity the caller
may not read. The damage is a wrong view: an operator-authored constraint
vanishes without a trace, which is a correctness and trust problem, not a
disclosure one.

## Fix — DECIDED (Jeroen, 2026-08-24; RULING 17)

Fail at **load** time, matching the automation precedent: validate every
view's `where:` when `data-entry.yaml` is parsed, and refuse to start on a
bad one.

A runtime 500 was explicitly considered and rejected as second-best: it makes
the failure visible but leaves the bad config deployable, so the operator only
learns when someone opens the view.

## Provenance

Found while analysing `views.go` for TKT-WRLDAPI item 4b (world-enabling
`_views`). Pre-existing, unrelated to worlds — filed separately so item 4b
does not silently inherit it. Related: [[TKT-WRLDAPI]].
