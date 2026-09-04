---
id: RR-GQWRLD
type: review-response
title: GraphQuery.World plumbed into the struct but dropped by the naive impl — ACL-path fail-open
finding: "PR-B added store.GraphQuery.World with a doc comment explaining it must exist so the list path and the ACL path cannot disagree — and then graphquerynaive never copied it into the EntityQuery it builds (naive.go collectByType and MatchingIDs). fsstore and memstore delegate all three GraphQueryer methods to that package, so on fs/mem a world-scoped GraphQuery silently ran unscoped. Reproduced on memstore with world(chain=[published], otherwise=exclude): ListEntities returns P1@published only, while GraphQuery on the SAME world returns P1@'' (the draft) AND P2@'' (which exclude should have dropped), with GraphCount matched=2 total=2 instead of 1. internal/visibility/pushdown.go routes any principal carrying a composed ACL query through GraphQuery, so the degradation hit precisely the ACL-GATED principals: drafts leak into a published world and `otherwise: exclude` stops hiding. Fail-open, in exactly the direction the zero Fallback was chosen to avoid. pgstore was unaffected only by accident (its PR-B refusal) — which means PR-C, whose job is to REMOVE that refusal, would have silently extended the fail-open to postgres."
severity: critical
status: addressed
resolution: "graphquerynaive.collectByType now takes the world and passes it into the seed EntityQuery, and MatchingIDs does the same; the relation walks in matches() deliberately stay unscoped per GraphQuery.World's own doc (who an entity is related to must not depend on the reader's world) and per the identity-anchor stance (Q5/RULING 4). Added conformance case Worlds/GraphQueryHonorsTheWorld exercising GraphQuery + GraphCount + MatchingIDs under a world — the suite previously covered only ListEntities/ListEntitiesPage/CountEntities, which is why 11 green cases coexisted with a total fail-open. Verified non-vacuous: reverting the fix fails all four assertions (wrong face served, excluded entity visible, both counts wrong, MatchingIDs true for an excluded entity). NOTE the complementary half is NOT in PR-B: internal/acl/readquery.go still builds GraphQuery values without a World, so the field is honored-but-never-set on that path until PR-D wires it. Raised to the architect for staging confirmation."
---

**Finding (code review, TKT-WAV8XP PR-B).**

Found by `/code-review`, reproduced independently before fixing.

This is the failure RR-7FOWDB predicted at design-review time ("once PR-B adds
`GraphQuery.World`, search passes a ZERO World and silently gets the default
world while the entity-list path beside it gets the requested one") — arriving
via a different route than expected: not search passing a zero World, but the
naive implementation discarding a World that WAS passed.

```
ListEntities(World=published, otherwise:exclude):  P1@"published" -> "P1 published"
GraphQuery  (World=published, otherwise:exclude):  P1@""          -> "P1 draft"        <- LEAK
                                                   P2@""          -> "P2 draft only"   <- should be EXCLUDED
GraphCount: matched=2 total=2  (want 1)
```

The field's own doc comment states the requirement it failed to meet:

> the ACL read path swaps an EntityQuery for a GraphQuery the moment a policy
> query exists (internal/visibility/pushdown.go) ... A world carried on only one
> of the two would make the list path and the single-entity path disagree.

**Sequencing consequence.** pgstore's PR-B refusal masked this. PR-C removes
that refusal, so this had a live path to reaching postgres unnoticed.
