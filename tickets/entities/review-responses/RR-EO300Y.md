---
id: RR-EO300Y
type: review-response
title: 'Relation affordance gate was missing on the create path: an edge refused by POST /relations/ was writable by riding the create body'
finding: 'validateRelationsModernAffordances had exactly one caller repo-wide, in handleV1UpdateEntity (write_handler.go:686). handleV1CreateEntity never called it, so a relation declared RelationVerdict{Creatable: false} was refused 403 on POST /{plural}/{id}/relations/{rel} but written anyway when the edge rode the create body''s relations: field. Demonstrated with a test before fixing: the create returned 201 and its own response body reported _relations: {"implements": {"creatable": false}} for the edge it had just written — the affordance map contradicting itself in the payload that produced it. Pre-existing, but TKT-R4BMJM makes it reachable from a button: a section create on an OUTGOING relation resolves to link_as: to, which is exactly the direction that rides the create body. The plan''s security item 2 asserted both directions were gated "by two different functions", and AC10 required asserting refusal on both; only one was true and the AC10 test did not exist.'
severity: critical
resolution: 'Added the gate to handleV1CreateEntity, placed on the existing pre-create `candidate` entity rather than beside the Phase A relation validation. Position matters: Phase A runs AFTER CreateEntity, so refusing there would leave the entity written and unlinked — turning an authorization denial into an orphan. No id is needed because verdicts resolve against type and properties, which the candidate carries. AC10''s test now exists (TestSectionCreate_EdgeRefusedOnBothLinkDirections) with three cases: both directions refused against a DECLARED non-creatable verdict, plus a creatable-verdict positive so the gate cannot refuse the ordinary case. The verdict is declared explicitly because validateRelationOp is default-permissive for undeclared types, which is the vacuous-pass trap the plan warned about.'
status: addressed
---

## Finding

`validateRelationsModernAffordances` had **exactly one caller** repo-wide:

```
internal/dataentry/write_handler.go:686:  if denial := h.affordances.validateRelationsModernAffordances(
```

That is `handleV1UpdateEntity` (PATCH). `handleV1CreateEntity` never called it.
So a relation declared `RelationVerdict{Creatable: false}`:

- `POST /{plural}/{id}/relations/{rel}` → **403**, `relation-affordance:not-creatable`
- the same edge riding the create body's `relations:` → **201, edge written**

Demonstrated before fixing. The create response reported, for the edge it had
just written:

```json
"_relations":{"implements":{"creatable":false,"removable":false}}
```

The affordance map contradicting itself inside the payload that produced it.

Pre-existing, but this ticket makes it reachable **from a button**: a section
create on an outgoing relation resolves to `linkAs: "to"`, and that is precisely
the direction that rides the create body. So the PATCH gate was bypassable by
routing an edge through a create instead of an update.

The plan's Security Considerations item 2 asserted both directions were gated
"by two different functions", and AC10 required asserting refusal on both. Only
one was true, and the AC10 test did not exist — exactly what RR-YNZBKN predicted
would happen if it were written carelessly.

## Resolution

Gate added to `handleV1CreateEntity`, on the existing pre-create `candidate`
entity rather than beside the Phase A relation validation.

**Position is load-bearing.** Phase A runs *after* `CreateEntity`, so refusing
there would leave the new entity written and unlinked — converting an
authorization denial into an orphan. The candidate carries type and properties,
which is all verdict resolution needs; no id is required.

`TestSectionCreate_EdgeRefusedOnBothLinkDirections` now covers AC10 with three
cases: both directions refused against a **declared** non-creatable verdict,
plus a creatable-verdict positive so the gate cannot refuse the ordinary case.
The verdict is declared explicitly because `validateRelationOp` is
default-permissive for undeclared types — the vacuous-pass trap the plan called
out.
