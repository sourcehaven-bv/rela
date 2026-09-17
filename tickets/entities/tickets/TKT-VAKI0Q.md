---
id: TKT-VAKI0Q
type: ticket
title: One scoped-read funnel for data-entry collection reads (the ACL verdict switch is copied four times)
kind: refactor
priority: high
effort: m
status: done
---

Extract the ACL read-verdict switch that `internal/dataentry` currently
reimplements in four places into one function, so a narrowing dimension is
threaded once instead of four times. Behaviour-preserving; no new feature.

## Problem

Four independent sites decide "AllowAll → `EntityQuery`, otherwise →
`GraphQuery` from the ACL verdict", then stamp the narrowings each knows about:

| site | file:line | carries |
| --- | --- | --- |
| `scopedSortedEntities` | `internal/dataentry/api_v1.go:368` | world, faces |
| `visibleEntitiesOfType` | `internal/dataentry/helpers.go:547` | world, props |
| `feedEntitySource.listType` | `internal/dataentry/feed_handler.go:150` | world |
| gantt (two paths) | `gantt_handler.go:311`, `:531` | world |

They were added on four different dates across eight months, each with a feature
that needed a collection read. Nothing is shared.

**This has already caused two fail-open bugs, both of the same shape.** The
comment at `api_v1.go:390` records them:

- **RR-GQWRLD** — `GraphQuery.World` was carried on one verdict branch only,
so the AllowAll population silently fell back to the default world.
- **TKT-O7R2A1** — the same for `FaceIn`, leaving the most privileged
population with no face narrowing.

The duplication is not superficial. `visibleEntitiesOfType` carries a comment
saying *"World scope on BOTH verdict branches, for the RR-GQWRLD reason
`scopedSortedEntities` spells out"* — the same invariant, restated by hand,
because there was nowhere to put it once.

Each new narrowing repeats the exercise, and a miss fails OPEN (shows rows that
should be hidden) rather than closed. Query scopes (TKT-EVR2TU) would be the
third dimension to run this gauntlet.

## Approach

One function owning the verdict switch and every narrowing:

```go
// ScopeRequest is what a collection read narrows by.
type ScopeRequest struct {
    Type   string
    Props  []store.PropPredicate  // extra conjuncts (search prefilters, …)
    // world and faces are read from ctx / the ACL result inside
}

func scopedHeaders(ctx, svc Services, rqr acl.ReadQueryResult, req ScopeRequest) ([]store.EntityHeader, error)
```

Migrate all four call sites onto it. The narrowings become fields on one struct,
so adding one is a compile-time prompt at a single site rather than a grep
across handlers.

Deliberately NOT in scope: changing what any surface returns. This is an
extraction. Every existing test must pass unchanged — that is the proof.

## Acceptance criteria

**AC1 — one verdict switch.** After the change, exactly one function in
`internal/dataentry` branches on `rqr.AllowAll` / `rqr.Query == nil` / `default`
for a collection read. A guard test scans the package and fails on a second one,
with an exemption list (the pattern `ceilingguard_test.go` uses).

**AC2 — both branches carry every narrowing.** A table test asserts, per
narrowing dimension, that the AllowAll and ACL-gated branches produce the same
narrowing. This is the RR-GQWRLD/TKT-O7R2A1 regression, pinned once for all
dimensions instead of once per dimension.

**AC3 — no behaviour change.** The existing data-entry suite passes with no
modifications. Any test that has to change is evidence the extraction altered
behaviour and must be justified in review.

**AC4 — the zero-verdict defence survives.** `scopedSortedEntities` currently
fails loud on a zero `ReadQueryResult` ("Defensive: a zero ReadQueryResult would
otherwise alias AllowAll"). That check moves into the funnel and keeps a test.

**AC5 — the ReadQueryResult copy survives.** The funnel must COPY before
stamping (`wq := *rqr.Query`), because the ACL layer may cache a result per
principal; mutating in place leaks one request's narrowing into the next. Pinned
by a test that calls the funnel twice with a shared result.

## Risks

- **An extraction that quietly changes behaviour is worse than the
duplication**, because four sites that differ visibly are safer than one that
differs invisibly. Mitigated by AC3: no test may change.
- **The four sites differ in return type** (entities vs headers vs iterators).
The funnel should return headers and let callers adapt, since `headerEntity` is
already the shared adapter.
- **The gantt has two internal paths that must agree** (full forest, drilled
subtree). Both migrate, or drilling changes the answer.
