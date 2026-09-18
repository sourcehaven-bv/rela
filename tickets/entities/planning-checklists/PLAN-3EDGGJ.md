---
id: PLAN-3EDGGJ
type: planning-checklist
title: 'Planning: Validation relation gates: a consumer-side graph seam, with direction and target-type filters'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: (1) a consumer-side graph interface declared in `internal/validation`,
replacing the two ad-hoc read paths the shipped gate uses; (2) `direction:` and
`target_type:` on `RelationConstraint`; (3) load-time validation of both new
keys plus `where`-against-`target_type`; (4) docs.

OUT: `world:` on a rule (needs its own subject-vs-target decision and an
arch-fence change); face scoping of target resolution (belongs with the world
decision — must be UNCHANGED here, pinned by a test); batching the per-edge
`GetEntity` (a performance change must not ride along with a semantics change);
a predicate/expression form; relation-property `where`.

**Acceptance Criteria:** as listed on TKT-NA2O5G. Each maps to a test in the
Test Plan below.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — the design question was "which existing in-repo pattern
applies", answered by direct inspection rather than a survey.

**Existing Solutions:**

- **`acl.Graph` (`internal/acl/graph.go:38`)** — the pattern adopted.
Consumer-declared graph interface, returns `[]string` (no store types),
store-backed `StoreGraph` in production, `NullGraph` for tests. Its doc comments
also settle the error-handling question: `HasEdge` returns false on backend
error (deny by absence) while `OutgoingRelations` propagates, "because a
principal-resolution that proceeds with partial data is worse than failing the
request loud." Validation's gate wants the second discipline.
- **`affordances.RelationLookup` (`internal/affordances/bindings.go:27`)** and
**`statemachine.GraphLookup` (`internal/statemachine/statemachine.go:99`)** —
two more consumer-side graph interfaces, both `OutgoingCounts`-shaped. Rejected
as a model: counts-only cannot answer a `where` on the target, and both are
outgoing-only, which is the restriction being lifted. RR-4XPL8N already records
their adapters drifting apart, which is an argument for not adding a fourth of
the same shape.
- **`lua.ReadDeps.OutgoingRelations` (`internal/lua/deps.go:110`)** — what the
shipped gate uses. Not an interface validation declares; a helper on a
capability bundle. One production caller. To be retired.
- **`analysis.countRelationsFor` (`internal/analysis/analysis.go:504`)** — the
face-correct counting precedent, using `RelationQuery.FromFace` for
content-scoped relations. Deliberately NOT adopted here (out of scope), but
noted as the reference for whoever does the world ticket.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

*Open question resolved: `where` validation when `target_type` is NOT set.*
The plan validates `where` properties at load only when `target_type` fixes a
single target type. Left there, the unset case is inconsistent — same rule
shape, different error timing. It is not an oversight but it must be stated:
when a relation reaches several types, a `where` property valid for one and
invalid for another has no single load-time answer, and rejecting it would
break the existing ability to write one gate across heterogeneous targets.

DECISION: when `target_type` is unset, validate the `where` properties
against the UNION of the relation's reachable types on the relevant side
(`RelationDef.From`/`To`, guaranteed non-empty by `loader.go:562-569`) and fail
at load only if the property exists on NONE of them.

Design-review S2 sharpens WHY this matters, with a case the ticket's own
motivating example walks up to: a relation reaching `taak` and `terugkerend`
where only one declares `status`, filtered by `where: ["status!=gereed"]`.
`MatchAll` errors for every `terugkerend` target, and under `max:` an
unevaluable target COUNTS AS MATCHING — so a `max: 0` gate fires on entities
that have only schedules. Partial-presence is therefore not merely untidy; it
silently inverts a gate. Emit a load-time WARNING when a `where` property is
missing from SOME but not all reachable types, naming them, so the operator is
told to set `target_type`. That catches the
real typo (a property no possible target has) without rejecting the legitimate
heterogeneous case, and it makes the unset path strictly better than today
rather than merely unchanged. A property present on some types and absent on
others keeps today's per-entity check-time behaviour, which is correct: the
fail-closed logic already handles it per target.


Two commits, deliberately ordered.

*Commit 1 — the seam, no behaviour change.* Declare in `internal/validation` a
narrow interface. The shape is constrained by two things the shipped code does
that a naive "return the far entities" interface would BREAK:

1. With `where` empty the gate counts EDGES and never reads a target at all
   (`validation.go:578`). Forcing target resolution would add reads and could
   change counts.
2. An edge whose target cannot be read still counts when `Max` is set
   (`validation.go:589-595`). If the adapter returned only successfully
   resolved targets, an unreadable one would vanish from the slice and a
   `max: 0` gate would silently report satisfied — the exact failure
   `TestRelationConstraint_UnevaluableTargetFailsClosed`
   (`internal/validation/relation_constraint_test.go:229`) exists to prevent.

So the interface must preserve a THREE-way distinction — edge with readable
target / edge with unreadable target / no edge — rather than collapsing to a
list of targets. Sketch:

    // one element per EDGE, in graph order
    type Related struct {
        ID    string         // far entity id, direction already applied
        Type  string         // "" when the target could not be resolved
        Props map[string]any // nil when unresolved
        Err   error          // non-nil when the target could not be read
    }
    RelatedEntities(ctx, subjectID, relType string, dir Direction) ([]Related, error)

*Multiplicity is part of the contract (design-review C1).* Relation identity is
`(From, FromFace, Type, To)`, not the triple — `entity.Relation.Key`
(`internal/entity/entity.go:302-308`): "two edges on the same triple with
different tails are two relations". A `RelationQuery` with `FromFace == nil`
leaves the tail unfiltered, so one subject CAN have N edges to the same target.
Today's loop counts EDGES (`validation.go:577-582`), so N face-tailed edges to
one target count N.

The per-edge shape above preserves this by construction, but the invariant must
be stated in the interface godoc — "ONE ELEMENT PER EDGE; ids may repeat" —
because the deferred batching work is exactly what would silently collapse it
to a set. Pinned by a test: two face-tailed edges to one target must count 2.

`Err` on the element is what carries "edge exists, target unreadable" so the
evaluator keeps its per-bound fail-closed choice. The outer error stays for
"the relation query itself failed", which is today's `LoadError` path.

Resolution stays LAZY where it is today: the adapter may skip reading targets
the caller will not inspect. Simplest correct form is for the adapter to
resolve targets always and the evaluator to ignore them when `where` is empty
— but that adds a read per edge on the empty-`where` path, which is most of
the 14 shipped gates. Prefer a resolve-on-demand shape (an accessor the
evaluator calls only when it has filters) or pass the "do I need targets"
decision in. Decide during implementation; the criterion is that the
empty-`where` path issues no target reads, pinned by a counting test.

`Related` is a validation-local value — no store types, so the arch fence
holds. Direction is a validation-local enum. The store-backed adapter lives in
`internal/validator` (which may depend on `store`), reading through the same
`VisibleReader` the gate uses today so the visibility story is unchanged. Wire
it at `validator.New` and `analysis.newValidationService`.
`checkRelationConstraint` then calls the interface instead of
`deps.OutgoingRelations` + `VisibleReader.GetEntity`. The 14 shipped gates must
behave identically; their existing tests are the gate.

*The incoming query MUST use EntityID, never From (design-review C4).*
`store.RelationQuery.Direction` gates ONLY `EntityID`/`EntityIDs`. `From` and
`To` are matched unconditionally and independently
(`internal/store/storeutil/storeutil.go:327-348`; `endpointMatches` at `:354`).
There is no `ValidateRelationQuery`, so a contradictory query is not rejected.

Verified empirically against `storeutil.NewRelationMatcher` with one edge A→B:
`{From:"B", Direction:Incoming}` matches NOTHING (B's incoming edge invisible);
`{From:"A", Direction:Incoming}` still matches A→B, an OUTGOING edge;
`{EntityID:"B", Direction:Incoming}` matches A→B, correct.

Today's query (`internal/lua/deps.go:113-117`) passes `From: fromID` AND
`Direction: DirectionOutgoing`, where the Direction is REDUNDANT — `From` alone
does the work. So the minimal-looking edit (keep `From`, flip `Direction`)
silently returns outgoing edges: it compiles, runs, and yields plausible
numbers.

CONSTRAINT: the adapter spells incoming as `{EntityID: id, Direction:
DirectionIncoming}` (or `{To: id}`), never `{From: id, Direction: ...}`.

This also corrects the "far end" mitigation above: picking `rel.From` vs
`rel.To` correctly is worthless if the QUERY returned the wrong edge set. The
direction test must assert the returned edge SET, not merely which endpoint was
read from it — a test that only swaps endpoint-picking would still pass against
the wrong query.

*Incoming counts are entity-level, and that must be stated (design-review S3).*
`entity.Relation` has `FromFace` and deliberately NO `ToFace`
(`internal/entity/entity.go:263-265`), so an incoming edge cannot be attributed
to a face of the entity it arrives at. `internal/analysis/analysis.go:504-517`
already establishes the asymmetry as precedent: the tail filter applies only on
the OUTGOING direction, "an incoming bound counts edges arriving at the entity
regardless of which state they left".

Because `loadCandidates` validates every face as its own row
(`validator.go`, `AllStates: true`), an `incoming` constraint on a 3-face entity
evaluates identically three times and emits THREE violations for one logical
defect. The plan's "face behaviour unchanged" criterion is inapplicable here —
`incoming` does not exist today, so there is no behaviour to preserve.

DECISION: allow it, and document that incoming counts are entity-level, not
per-face. Do NOT make it a load error: the atlas motivating case is exactly an
incoming constraint on a faced type (`procedure` with `concept`/`vastgesteld`),
so rejecting it would block the use case the ticket exists for. Accept the
duplicate reporting for now; deduplicating per-entity is the world ticket's
business, not this one's. Pinned by a test asserting the count is
face-independent, and named in the godoc + `docs/metamodel.md`.

*Symmetric relations: reject, do not accept (design-review S4, corrected).*
A symmetric relation is ONE stored row; no reciprocal edge is ever written
(`Symmetric` appears nowhere in `internal/store/**` or
`internal/entitymanager/**`; `Manager.CreateRelation` makes exactly one store
call). Symmetry is a read-time presentation convention only
(`internal/dataentry/default_view.go:92-95` skips the incoming pass precisely
to avoid double-counting the same edges). So for symmetric type T with a stored
edge A→T→B, a query from B finds nothing: A and B would get DIFFERENT counts
for the same relationship, decided by write order. This supersedes my earlier
"treat both directions as equivalent" note: make `direction:` on a
`symmetric: true` relation a LOAD ERROR.

*Adapter home: a new leaf package (design-review S1, DECIDED).*
The plan originally put the store-backed adapter in `internal/validator`. That
is unbuildable: `.go-arch-lint.yml` `analysis.mayDependOn` lists
`frontmatter, lua, metamodel, project, schema, storage, store, tracer,
validation` — no `validator` — and `analysis.newValidationService` is the second
entry point into `validation.Service`. `lua` being the only package BOTH can
reach is precisely why `OutgoingRelations` ended up on `ReadDeps`.

DECISION: a new leaf package (working name `internal/validationgraph`) holding
the interface's store-backed implementation. Arch-lint changes: declare the
component, give it `mayDependOn: [entity, metamodel, store]`, and add it to
`validator` and `analysis`. `validation` itself does NOT depend on it — it
declares the interface and receives an implementation, so the criterion
"`internal/validation` must not import `internal/store`" still holds.

Rejected: adding `validator` to `analysis.mayDependOn` (a new dependency edge
between two peers, when a shared leaf is the pattern the repo already uses);
widening `ReadDeps` in place (keeps the non-seam shape this ticket exists to
correct).

*Commit 2 — the keys.* `Direction` and `TargetType` on `RelationConstraint`,
passed through to the interface (direction) and applied as a pre-`where` skip
(target type). Loader checks added to the existing `validateValidationRelations`
(`internal/metamodel/loader.go:1563`), which already validates relation-type
existence and bound satisfiability in exactly this style.

Why the far-end choice lives INSIDE the adapter: `rel.To` is correct for
outgoing and wrong for incoming, and the wrong choice yields plausible-looking
numbers rather than an obvious failure. Returning "the far entities" rather than
edges makes the mistake unavailable instead of merely tested for.

Alternatives rejected: widening `lua.ReadDeps.OutgoingRelations` in place (keeps
the non-seam shape that caused the restriction); a counts-only lookup like
affordances/statemachine (cannot answer `where`); an expression form (the
declarative block shipped — two syntaxes for one job is worse).

**Files to modify:**

- `internal/validation/` — interface, direction-aware evaluation, target-type filter
- `internal/validator/` — store-backed adapter + wiring
- `internal/analysis/analysis.go` — wiring (second entry point)
- `internal/appbuild/` — wiring
- `internal/metamodel/types.go` — two fields on `RelationConstraint`
- `internal/metamodel/loader.go` — extend `validateValidationRelations`
- `internal/lua/deps.go` — retire `OutgoingRelations` if unused after
- `docs/metamodel.md`, `docs-project/entities/guides/GUIDE-metamodel.md`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `direction:` — operator-authored `schema.yaml`. Allowlist of exactly two
words; anything else is a load error. Not a blocklist.
- `target_type:` — operator-authored. Must be a declared entity type AND
reachable by the keyed relation on the relevant side (`To` for outgoing, `From`
for incoming). Load error otherwise, because a type the relation cannot reach
makes the gate pass forever.
- `where:` — unchanged syntax; when `target_type` is set its properties are
additionally checked against that type's definition at load.

Config is not secret (per CLAUDE.md), so load errors naming relation types,
entity types and property names are appropriate and useful.

**Security-Sensitive Operations:**

Read gating. The gate counts what the acting identity can SEE. This is settled
and CORRECT, not a defect: validation runs over an ACL-pruned graph, so a
principal may legitimately not see issues a fuller view would show. A gate is a
statement about the visible graph, never a global invariant. The CLI and CI
paths wire an unrestricted reader, so the verdict that enforces the workflow
sees everything. `RelationConstraint`'s godoc
(`internal/metamodel/types.go:1504-1517`) and `docs/metamodel.md` both already
say this accurately.

The adapter must therefore keep reading through `VisibleReader`. Taking a raw
store handle would silently turn a visibility-scoped gate into a global one and
leak hidden entities through violation counts.

*Design-review C3, resolved.* The review claimed the `failClosed` branch is
"substantially dead" because `visibility.PolicyReader.FilterRelations`
(`internal/visibility/policyreader.go:161`) drops an edge when either endpoint
is hidden, BEFORE the evaluator runs — so an ACL-hidden target never reaches
the `GetEntity` failure path. The mechanism is real and verified. The
conclusion that it is a soundness hole is NOT accepted: pruning is the intended
semantics per the paragraph above.

Two corrections to the review: `types.go:1504-1517` is already accurate (it
states the drop and that it matters mainly for Max), and the comment at
`validation.go:583-590` is accurate for its own scope — it describes edges that
REACH the loop. Neither is a lie.

What IS worth fixing, and is now in scope: `validation.go:583-590` reads as a
general guarantee without noting that ACL pruning already happened upstream. Add
one cross-referencing sentence so the next reader does not infer a stronger
property than the read path delivers. The `failClosed` branch still genuinely
covers dangling edges and `MatchAll` errors, so it stays.

Consequently the plan does NOT claim to "preserve fail-closed on invisible
targets" — that was never true. It claims: behaviour unchanged, including the
pruning, pinned by tests.

Fail-closed semantics that DO exist must survive the move: an unevaluable
target (dangling edge, or a `MatchAll` error) counts as matching when `Max` is
set and is skipped otherwise, and a constraint that cannot run is reported as a
`LoadError` rather than silently passing. Losing either converts a gate into a
no-op.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| Criterion | Test |
| --- | --- |
| One graph seam | Grep-style guard: the relation gate has no `OutgoingRelations` / `VisibleReader.GetEntity` call. Plus `just arch-lint`. |
| No store types in the interface | `just arch-lint` (validation must not import `internal/store`). |
| 14 gates unchanged | Existing gate tests pass at commit 1 AND commit 2, unmodified. |
| `direction: incoming` | Fixture A→B where the constraint is on B. Outgoing counts 0, incoming counts 1. A test that only counted would pass with the endpoints swapped; this one asserts WHICH entity was resolved. |
| direction omitted | Existing tests, unmodified. |
| `target_type` narrows | Relation reaching two types, one target of each; `target_type` on one yields count 1. |
| `where` vs `target_type` at load | Unknown property with `target_type` set = load error, not per-entity check error. |
| invalid `direction` | Load error naming the bad value. |
| unreachable `target_type` | Load error; separately, a type that IS declared but not reachable by this relation still errors. |
| fail-closed preserved | Existing tests for the Max/Min unevaluable-target split, unmodified. |
| unreadable target still counts under Max | NEW test on the `GetEntity`-error path specifically (the existing test covers the `MatchAll`-error path). Adapter returns an edge whose target cannot be read; `max: 0` must still violate. This is the regression the seam most likely introduces. |
| empty `where` issues no target reads | Counting adapter asserts zero target resolutions for a constraint with no `where`. |
| face behaviour unchanged | Faced fixture; count matches pre-change behaviour exactly. |
| atlas rules expressible | Fixture mirroring `procedure`/`taak`/`terugkerend` over an incoming multi-target-type relation. |

Integration: the tickets project's own 14 gates are a regression check — `rela
validate` over `tickets/` must report identically before and after.

*But it is NOT sufficient (design-review C2).* `tickets/schema.yaml` declares
ZERO faces (`grep -c "faces:" tickets/schema.yaml` → 0) and no `scope: content`
relations, so it exercises only the faceless, one-edge-per-triple case — the one
case where C1 cannot bite and where a "face behaviour unchanged" criterion
passes vacuously. A criterion satisfied by construction rather than
verification is the "clean run over data it never looked at" failure this
codebase repeatedly warns about (`metamodel/types.go:96-112`,
`loader.go:validateValidationFaces`).

So the face and multiplicity criteria need a purpose-built fixture with a faced
type and a face-tailed relation. `internal/validator/faces_test.go` already
exists and is the place to extend.

**Edge Cases:**

- Self-referencing relation (from-type == to-type): direction still
distinguishes the two ends; both must be counted correctly. Precedent:
`internal/dataentry/default_view.go:96-101` treats a self-referential
relation as genuinely two-directional ("the edges visible to the inverse are
different from the outgoing"), so both directions are meaningful and neither
is a duplicate.
- Symmetric relation (`symmetric: true`): follow the established convention
rather than inventing one. `internal/dataentry/relations_direction.go:49-55`
treats a symmetric relation as OUTGOING by convention ("the metamodel's
`symmetric: true` flag tells the reconciler the edge has no preferred
direction") and `default_view.go:92-95` skips the incoming pass because it
"would duplicate the same edges". So for a symmetric relation the two
directions are the same set. DECISION: accept `direction:` on a symmetric
relation and treat both values as equivalent, matching that convention —
do NOT reject it, and do NOT double-count. Pinned by a test.
- `target_type` set with an empty `where` — type filter alone must work.
- Zero edges, with `min: 0` and with `max: 0`.
- Target invisible to the acting identity — must keep today's per-bound
fail-closed behaviour, not the new code's convenience.
- A relation whose `From`/`To` lists are empty or omitted.

**Negative Tests:**

`direction: sideways`, `direction: ""` (explicit empty vs absent), `target_type:
nonexistent`, `target_type` declared but unreachable by the relation, `where`
naming a property the `target_type` lacks. All must be LOAD errors with messages
naming the rule, the relation and the offending value — matching the style
already in `validateValidationRelations`.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *Wrong endpoint on incoming edges* — plausible-looking wrong numbers.
Mitigated structurally (adapter returns far entities, not edges) and by a test
that asserts identity, not count.
- *Regressing the 14 shipped gates during the move* — mitigated by making
commit 1 behaviour-preserving with the existing tests as the gate, before any
new feature lands.
- *Losing the fail-closed semantics in the refactor* — explicit criterion plus
the existing tests.
- *Scope creep into worlds/faces/batching* — each is out of scope with a
criterion pinning "unchanged" rather than "improved".

*Design-review minors, accepted (M1-M4).*

- **M1** — the ticket's stated symptom was wrong and is corrected: a `where`
  that fails to PARSE is already caught once per rule
  (`validation.go:127-140`). `type=taak` PARSES and fails inside `MatchAll`,
  swallowed by the fail-closed branch — so the real symptom is SILENCE (no
  diagnostic), not repeated errors. Strengthens the motivation.
- **M2** — `GetEntityDef` resolves aliases (`metamodel.go:39-50`) but
  `validateRelationReferences` checks `m.Entities` directly
  (`loader.go:571,577`). An aliased `target_type` would pass at check time and
  fail the new load-time check. Normalize through `ResolveAlias` on both sides;
  negative test for an aliased `target_type`.
- **M3** — "OUTGOING" is hardcoded in prose in TWO godocs
  (`types.go:1495-1497` and `types.go:157-161`). `commentlint`'s `duplication`
  rule targets exactly this. Both must be updated; the ticket's file list
  omitted `types.go:157`.
- **M4** — `validator.New` has FOUR production call sites, not three:
  `appbuild.go:574`, `appbuild.go:1801`, `appbuildtest/fixture.go:250`,
  `dataentry/app.go:1057`. The last uses a per-request `lateGatedReader`
  (`app.go:1048`) — the only site where the ACL story is non-trivial, and the
  one the plan omitted.

**Effort:** l (re-estimated from m after design review; the original estimate
predated C1-C4 and S1-S5)

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs/metamodel.md` — extend the shipped "Relation Cardinality
Validation" section with `direction:` and `target_type:`, including the note
that `where` is validated at load when `target_type` is set.
- [x] `docs-project/entities/guides/GUIDE-metamodel.md` — the mirrored copy
updated in the same commit (TKT-R8QEU updated both).
- [x] ~~CLAUDE.md~~ (N/A: no new cross-cutting convention; the seam follows the
existing consumer-side-interface rule already documented there)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 4 critical, 5 significant, 5 minor. All critical and
significant findings are addressed in this plan and the ticket; each has a
review-response entity linked to TKT-NA2O5G.

- RR-CE7JVU (critical) — flat `[]Target` loses edge multiplicity → per-edge
  return with a stated "ids may repeat" invariant + pinning test.
- RR-WXZ00T (critical) — `Direction` gates only `EntityID`, so `{From, Incoming}`
  silently returns OUTGOING edges → hard constraint on the query spelling,
  verified empirically; test must assert the edge SET.
- RR-YTRVNW (critical) — plan claimed a `Max` fail-closed guarantee that does
  not hold for ACL-hidden targets → mechanism accepted, "soundness hole"
  conclusion REJECTED (pruning is intended); claim corrected to
  "behaviour unchanged", plus one clarifying comment in scope.
- RR-MSLJHN (critical) — `tickets/` has zero faces so the named integration test
  passed vacuously → purpose-built faced fixture required.
- RR-CNM4HB (significant) — `analysis` cannot import `validator`, so the
  proposed adapter home was unbuildable → new leaf package.
- RR-M2XRW2 (significant) — partial-presence `where` on a multi-type relation
  silently inverts a `max` gate → validate against the reachable-type union,
  warn on partial presence.
- RR-WCH0E6 (significant) — `direction:` on a symmetric relation gives the two
  endpoints different counts → load error.
- RR-LS7AG1 (significant) — incoming on a faced type reports once per face →
  allowed and documented as entity-level; not a load error, since the atlas case
  needs it.

Minors M1-M4 accepted and folded in (see Risk Assessment). Effort re-estimated
m → l.
