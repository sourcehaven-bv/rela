---
id: RR-YNZBKN
type: review-response
title: 'AC10 would have passed vacuously: default-permissive relation verdicts, and only one of two code paths tested'
finding: 'Two problems with AC10 as originally written. First, validateRelationOp returns nil for relation types with no verdict entry (affordances.go:608-611, default-permissive), so a contract test on a schema declaring no relation verdicts passes trivially and proves nothing about edge refusal. Second, the two link_as values take different server code paths: link_as=to rides the create body through validateRelationsModernAffordances, while link_as=from is a separate POST through validateRelationOp (write_handler.go:947). Both gate, but via different functions with different error shapes, and nothing tested refusal on either.'
severity: significant
resolution: AC10 rewritten to require edge refusal asserted on both link_as directions against a schema that declares a non-creatable relation verdict. Both the ticket and the plan's test table now state why a verdict-free schema would pass vacuously, so the constraint survives a later edit.
status: addressed
---

## Finding

Two problems with AC10 as originally written.

**It could not fail on the relation half.** `validateRelationOp` returns `nil`
for a relation type with no verdict entry (`affordances.go:608-611`, explicitly
commented `default-permissive`). A contract test against a schema declaring no
relation verdicts therefore passes trivially while exercising nothing.

**It tested one of two code paths.** The two `link_as` values diverge
server-side: `link_as: to` rides the create body through
`validateRelationsModernAffordances`, while `link_as: from` is a separate `POST
.../relations/{relType}` through `validateRelationOp` (`write_handler.go:947`).
Both gate, but via different functions with different error shapes — so they can
drift. AC5 tested direction *correctness*; nothing tested direction *refusal*.

## Resolution

AC10 rewritten on both the ticket and the plan: edge refusal must be asserted on
**both** `link_as: to` and `link_as: from`, against a schema that declares a
non-creatable relation verdict. Both places state *why* a verdict-free schema
would pass vacuously, so the constraint survives someone later simplifying the
fixture.
