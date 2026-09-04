---
id: RR-CGRV0X
type: review-response
title: RelationQuery.FromFace nil-is-unfiltered is fail-open under a world; scope dispatch is unowned
finding: 'store.go:368-375 documents FromFace nil as leaving the tail UNFILTERED (every state''s edges plus identity edges). Correct for DOFYR1 compat, fail-open under a world: a world-resolved read that omits FromFace mixes draft and published edges, and §2.3 makes content-scoped edges the carrier of a state''s outbound content. Whichever way Q4 lands, no version of the plan says WHO performs the per-relation-scope dispatch — identity-scoped edges must match with nil, content-scoped ones must match the prime''s face, and under FallbackDefaultState the prime''s face is the ZERO face, which is a different filter than nil.'
severity: significant
resolution: 'ARCHITECT DECISION (Q4): per-relation-type SCOPE DISPATCH at the decorator layer. RelationQuery gets NO world and NO new store-contract surface; DOFYR1''s contract and storetest''s pins stand frozen. The decorator holds both the metamodel (scope) and the world (prime), so it dispatches: identity-scoped types query with a nil tail, content-scoped types with the prime''s face, results merged. One resolution site, no second argmin. Placement: internal/worlds, whose arch-lint deps are already exactly entity+metamodel+store — no arch-lint change needed, and visibility (which may not import metamodel) is correctly excluded. Mandatory follow-through in PR-D: (1) the fallback trap — when otherwise:default fires the prime''s face IS the zero face, which as a FromFace value means default-tail-ONLY, a different filter from nil; content-scoped queries for a fallback prime are correctly &zero while identity-scoped must stay nil, both pinned in a named test; (2) the omission must be unrepresentable — the world-resolved surface hands out the relation-reading capability carrying the dispatch, with no way to issue a raw nil-tail query through it; (3) acceptance cases are tracer (7 nil-tail reads) and acl/storegraph.go:55''s role-conferral walk — pin that a world-resolved tracer does not see a non-prime state''s content edges and that identity edges remain visible from every face.'
status: addressed
---

**Finding (design review, TKT-WAV8XP PR-A planning).**

Flagged specifically as "made harder by starting PR-A before Q4 lands".

`internal/store/store.go:368-375`: `FromFace *entity.Face` nil "leaves the
tail UNFILTERED: edges from every state plus identity edges all match." Right
for DOFYR1 compat. Under a world it is a leak — a world-resolved read that
forgets `FromFace` mixes draft and published edges, and §2.3 makes
content-scoped edges the carrier of a state's actual outbound content ("a draft
may cite SPEC-12 while published still cites SPEC-9").

The survey named this correctly and deferred it to Q4. Two things are affected
in PR-A regardless of how Q4 lands:

1. PR-A introduces `worlds:` and per-type `faces:` with NO cross-check
against `metamodel.RelationScope` (`types.go:438-486`, declarative-only since
DOFYR1) — e.g. that a `content`-scoped relation's endpoints are types that
actually declare faces. That check is load-time-only, cheap, does not
pre-commit Q4, and becomes a migration once `worlds:` blocks exist in the wild.
2. Under `FallbackDefaultState` the prime's face is the ZERO face, which
as a `FromFace` value matches default-tail edges ONLY — a different filter
than nil. Whether that is correct depends on Q4, and the plan is silent.

**Resolution:** do not block PR-A. (a) Add the metamodel cross-validation to
PR-A's `validateWorlds` list. (b) Extend the plan's Q4 risk row to record that
`FromFace`'s nil semantics must be revisited whichever way Q4 lands, because
even the "resolver supplies FromFace" answer requires a per-relation-SCOPE
dispatch (identity → nil, content → the prime), not a single face value — and
no version of the plan currently assigns that responsibility.
