---
id: RR-CUUZ9Z
type: review-response
title: Single-entity Get path is never world-resolved in the plan
finding: The plan specifies world resolution only for LIST shapes (EntityQuery, GraphQuery, listPushdown, DISTINCT ON, pagination). GetEntity(id) is contractually the DEFAULT state (store.go:236-244) and is what visibility.PolicyReader.Get (policyreader.go:51) and all five tracer sites (tracer.go:100,156,200,239,267) call. A visibility ∘ world(published) reader whose Get is unresolved returns the DRAFT for a direct id fetch while its list path returns the published face — i.e. GET /api/v1/entities/{id} is the most direct read of exactly what a published world must hide.
severity: critical
status: addressed
resolution: "Architect decision 2026-08-20: ACCEPTED. GET is the dominant read and must resolve. Mechanism: resolution lives ENTIRELY in the decorator (store.EntityReader does NOT gain a world-aware get) - the store stays a raw addressable surface per the DOFYR1 contract, and one resolution site remains the invariant. The decorator's Get walks the chain via GetEntityState per coordinate (chains are 1-3; common case one call), applies the fallback verdict, and returns not-found under FallbackExclude, indistinguishable from a genuine miss and from an ACL denial. Get keeps its SIGNATURE (Q7 stands) but changes its SEMANTICS behind the world-resolved reader - the plan conflated the two. AC: the parity test must cover Get as well as list paths."
---

**Finding (design review, TKT-WAV8XP PR-A planning).**

Everything in the plan is list-shaped. But the dominant read is `GetEntity(ctx,
id)` on a bare id, and the plan addresses it nowhere.

Verified in tree:

- `internal/store/store.go:236-244` — `GetEntity(id) ≡ GetEntityState(id, zero
Face)`, the default state, unconditionally.
- `internal/visibility/policyreader.go:51` — `PolicyReader.Get` calls
`r.get.GetEntity(ctx, id)`.
- `internal/tracer/tracer.go:100,156,200,239,267` — five call sites.

So in PR-D a `visibility ∘ world(published)` reader whose `Get` path is not
world-resolved returns the DEFAULT state — the draft, under the design doc's own
`draft: {default: true}` example — for a direct id fetch, while its
`ListEntities` path correctly returns the published one. That is not a corner
case: it is `GET /api/v1/entities/{id}`.

The design doc §4.3 promise that "tracer, analysis, export and search run over a
resolved world unchanged" is only true if the reader interface they consume
resolves on EVERY method, including `Get`.

Note the plan's Q7 note ("existing `visibility.Reader` methods keep their
signatures, which is what lets Step 2 not touch 30 call sites") is fine as
stated — keeping `Get`'s SIGNATURE is right; keeping `Get`'s SEMANTICS is the
leak. The plan does not distinguish the two.

**Resolution:** state the mechanism in PR-B and add a PR-D acceptance criterion.
`Get` resolves the chain via `GetEntityState` per coordinate (chains are 1–3
long; the common case is one call), applies the fallback verdict, and returns
not-found under `FallbackExclude`. The not-found MUST be indistinguishable from
a genuine miss and from an ACL denial — the plan's Security §5 says this but
files it as an error-message rule, not a `Get`-path requirement. Also decide NOW
whether `store.EntityReader` gains a world-aware get or whether resolution lives
entirely in the decorator, since AC10's parity test as written compares only
list paths.
