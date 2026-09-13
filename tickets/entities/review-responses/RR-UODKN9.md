---
id: RR-UODKN9
type: review-response
title: 'Attribution map must dedupe per pass: traverse rules run up to 10 times in a fixpoint loop'
finding: 'Traverse rules run up to 10 times in a fixpoint loop (views.go:65-75). Collections survive this by deduping on merge (views.go:174-188), but a naively accumulated parent-child attribution map would append duplicate child ids per pass, doubling rollup counts on any multi-pass view. Additionally, traverseViewBreadthFirst (recursive: true) builds no bySource equivalent, so recursive + nested would silently lose attribution.'
severity: significant
resolution: 'Plan updated. Attribution map now specified to dedupe on merge, mirroring the existing collection dedupe at views.go:174-188, with test scenario 10 pinning no-double-counting via a multi-pass (self-referential containment) fixture. Separately, `recursive: true` + `display: nested` is now rejected at config load rather than silently losing attribution: this ticket renders exactly two levels so `recursive` has no meaning for it, and BFS-path attribution is deferred to a future multi-level ticket.'
status: addressed
---

## Finding

The plan proposes retaining `bySource` from `traverseViewMany` to get
parent→child attribution, describing it as "retain a mapping that is already
computed". Correct in principle, but the plan does not account for the
**fixpoint loop**.

`executeView` runs **every traverse rule up to 10 times** until the collection
sizes stop growing (`internal/dataentry/views.go:65-75`):

```go
maxPasses := 10
for range maxPasses {
    before := countViewEntities(result.Collections)
    for _, rule := range view.Traverse {
        h.applyViewTraverse(ctx, rule, result, w)
    }
    if countViewEntities(result.Collections) == before { break }
}
```

`applyViewTraverse` survives this because it explicitly dedupes by id when
merging into the collection (`views.go:174-188` — builds an `existing` set,
appends only unseen ids).

A naive `result.Attribution[collectAs][parentID] = append(..., childIDs...)`
would **not** dedupe, so on a view that needs two passes to stabilise, every
parent would list each child twice, and the rollup counts would double. The bug
would be invisible on a single-pass view (the common case) and appear only where
a later rule feeds an earlier one — exactly the hard-to-notice case.

## Required in the plan

State that the attribution map dedupes per `(parent, child)` on merge, mirroring
`views.go:174-188`, and add a test with a traverse that requires more than one
pass (a self-referential containment relation is the natural fixture) asserting
counts are not doubled.

## Also verify

`traverseViewBreadthFirst` (`views.go:233+`, used when `recursive: true`) builds
no `bySource` equivalent — it returns a flat id list level by level. So
`recursive: true` combined with `display: nested` would silently lose
attribution. Either reject that combination at config load, or build the mapping
in the BFS path too. The plan is silent on this and must not be, because
`recursive` is a documented traverse feature and the failure is silent.
