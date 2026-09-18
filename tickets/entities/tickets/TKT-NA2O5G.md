---
id: TKT-NA2O5G
type: ticket
title: 'Validation relation gates: a consumer-side graph seam, with direction and target-type filters'
kind: enhancement
priority: medium
effort: m
status: planning
---

## Description

TKT-R8QEU shipped the declarative `relations:` block on `ValidationRule` (PR
#1571). It covers the 14 workflow gates it was built for, but two restrictions
block a real use case found while adding `procedure` rules to atlas (ISMS) — and
the way the shipped code reaches the graph is what makes them awkward to lift.

### The underlying problem: validation has no graph seam

Validation reads the graph through two unrelated, ad-hoc paths:

- `lua.ReadDeps.OutgoingRelations` (`internal/lua/deps.go:110`) — a helper
hung off a **capability bundle**, not an interface validation declares. It has
exactly one production caller (`internal/validation/validation.go:572`).
- a raw `s.deps.VisibleReader.GetEntity` for each target
(`internal/validation/validation.go:592`).

Because the first is a convenience function rather than a designed seam, it
hardcodes `Direction: store.DirectionOutgoing` — direction was never a parameter
anybody had to think about. Widening it in place would keep the same shape and
store the next restriction in the same place.

The fix is the pattern `internal/acl` already uses: a **consumer-side graph
interface declared in `internal/validation`**, supplied by the wiring site
(`internal/acl/graph.go:38`, `Graph`, with `StoreGraph` in production and
`NullGraph` in tests). Validation then states what it needs from the graph and
knows nothing about how it is resolved.

This matters beyond tidiness. Face/world resolution — which face of a target
counts toward a gate — becomes a property of the injected adapter rather than a
concept the rule evaluator has to carry. `internal/validation` cannot import
`internal/worlds` under the current arch fence and should not need to: the
wiring site already knows the world and can hand over an adapter that resolves
in it. That is deliberately NOT built here (see Out of scope), but the seam is
what makes it possible later without touching the evaluator.

### 1. Only outgoing edges are counted

A constraint about edges arriving at the entity cannot be written at all.

Atlas needs "a procedure must have a periodic task" and "a procedure must have
an open task". Both are questions about the entity at the far end of an
**incoming** `gaat_over` edge. The edge points task → procedure, so from the
procedure there is nothing outgoing to count.

### 2. `where` cannot filter on the target's type

`filter.MatchAll` (`internal/filter/match.go:140`) resolves each clause's
property against `entityDef.Properties` and errors on anything else, so `where:
["type=taak"]` fails with `unknown property "type"`. Each target is also matched
against its **own** `targetDef`, so a relation accepting several source types
cannot be narrowed to one of them.

`gaat_over` accepts two source types (`taak`, `terugkerend`) across ten target
types. Counting the edges therefore does not say whether what was found is a
task or a schedule — which is precisely the distinction the rule is about.
`status` does not substitute for it: the two types use different status
vocabularies, so `where: ["status!=gereed"]` silently means different things
depending on which type happens to be at the far end.

The failure mode today is a per-entity `LoadError`
(`internal/validation/validation.go:521`), not a load-time error, so a rule with
an unusable `where` reports once per candidate entity at check time. The gate
correctly refuses to pass, but the diagnosis arrives late and repeated.

## Scope

**1. A consumer-side graph interface in `internal/validation`**, modelled on
`acl.Graph`: the minimum the relation gate needs, expressed in validation-local
terms, with the store-backed adapter supplied at the wiring site and a null
implementation for tests. Retire validation's use of
`lua.ReadDeps.OutgoingRelations` and its direct `VisibleReader.GetEntity` call
so the gate has ONE way to reach the graph.

`OutgoingRelations` has one production caller; if nothing else needs it after
this, delete it rather than leaving a misleading helper on `ReadDeps`.

**2. Two optional keys on `RelationConstraint`**
(`internal/metamodel/types.go:1518`), both defaulting to today's behaviour so
every shipped rule is unaffected:

```yaml
- name: procedure-heeft-open-taak
  entity_type: procedure
  faces: [vastgesteld]
  severity: error
  relations:
    gaat_over:
      direction: incoming   # NEW; default outgoing
      target_type: taak     # NEW; default any
      where:
        - "status!=gereed"
      min: 1
```

- `direction:` — `outgoing` (default) / `incoming`.
- `target_type:` — restricts counting to targets of that type, and fixes the
`entityDef` the `where` clauses resolve against. A separate key rather than a
magic `type=` inside `where`, because `where` is documented as filtering the
target's **properties**; it also gives the loader something to check.

**3. Load-time validation.** Reject a `direction` that is not one of the two
words, and a `target_type` the keyed relation cannot reach (the metamodel knows
each relation's `From`/`To` sets). A rule scoped to an impossible type matches
nothing and would pass forever — the silent no-op class that TKT-R8QEU's
unknown-key rejection exists to prevent. When `target_type` is set the target
definition is known statically, so validate the `where` properties against it at
LOAD time, turning today's repeated check-time `LoadError` into one error at
load.

**4. Docs** in `docs/metamodel.md` beside the shipped `relations:` reference,
and the mirrored `docs-project/entities/guides/GUIDE-metamodel.md`.

**Out of scope**

- **`world:` on a validation rule.** The seam in scope item 1 is what makes
this addable later without `internal/validation` importing `internal/worlds`.
Deferred deliberately: it needs its own decision about whether a world re-scopes
which SUBJECT rows are validated, which `loadCandidates` currently argues
against ("choosing ONE state to validate would be world resolution, a read-path
concern that has no business here") and which the store enforces by rejecting
`World` + `AllStates` together (`internal/store/store.go:322`). File separately.
- Face scoping of target resolution. Same reason — it belongs with the world
decision, and this ticket must not change it by accident. Whatever the gate does
today about faces, it must still do after this ticket, pinned by a test.
- A predicate/expression form of these gates. The declarative block shipped
and works. Lua remains the escape hatch for multi-hop walks and per-defect
messages.
- A face filter on the rule. `faces:` already covers it.
- Relation-property filters (`where` on the edge rather than the target).

## Approach

### The seam first, the keys second

Introduce the interface and move the existing behaviour onto it with no semantic
change — the 14 shipped gates must be byte-identical in behaviour at that
commit. Then add `direction` and `target_type` as parameters the interface
already has room for. Doing it in that order keeps the diff reviewable and means
a regression in step one is visible before any new feature is layered on.

Arch-lint: `validation.mayDependOn` has no `store`, so the interface must be
expressed in validation-local terms (ids, type names, property maps), not store
types — exactly as `acl.Graph` returns `[]string` rather than
`[]*entity.Relation`. The adapter lives where a store handle exists;
`internal/validator` may depend on `store` and is the natural home, with
`appbuild` and `analysis` wiring it.

Note both current entry points into `validation.Service` — `validator.New` and
`analysis.newValidationService` — will need to supply the adapter.

### Direction on the far end

With `DirectionIncoming` the far entity is `rel.From`, not `rel.To`.
`checkRelationConstraint` reads `rel.To` unconditionally (`validation.go:592`).
Getting this wrong counts real edges against the wrong entities and still
reports plausible-looking numbers, so it needs a test that distinguishes the two
ends rather than one that merely counts.

Putting the far-endpoint choice INSIDE the adapter (the interface returns "the
entities at the other end", direction already applied) is preferable to
returning edges and having the evaluator pick an end — it makes the mistake
unavailable rather than merely tested for.

### Preserve the fail-closed reasoning

The shipped code is careful: an unevaluable target counts as matching when `Max`
is set and is skipped otherwise, so each bound fails closed
(`validation.go:583-590`), and a constraint that cannot run is reported as a
`LoadError` rather than silently passing (`validation.go:517-524`). Carry this
across unchanged and keep its tests.

### Cost

`checkRelationConstraint` does one `GetEntity` per edge per entity, unbatched,
plus one relation query per entity per constraint. A `target_type` filter
slightly reduces it (skip before the fetch). The interface is the right place to
batch later — `store.RelationQuery.EntityIDs` exists for it — but batching is
NOT in scope here; size it separately rather than smuggling a performance change
into a semantics change.

### Files

- `internal/validation/` — the graph interface, direction-aware evaluation,
`target_type` filtering.
- `internal/validator/` — the store-backed adapter, wiring.
- `internal/analysis/` — wiring (second entry point into `validation.Service`).
- `internal/appbuild/` — wiring.
- `internal/metamodel/types.go` — the two fields on `RelationConstraint`.
- `internal/metamodel/loader.go` — validation of both keys and of `where`
against `target_type`.
- `internal/lua/deps.go` — retire `OutgoingRelations` if unused after.
- `docs/metamodel.md`, `docs-project/entities/guides/GUIDE-metamodel.md`.

## Acceptance Criteria

- [ ] `internal/validation` reaches the graph through ONE consumer-side
interface it declares; no `lua.ReadDeps.OutgoingRelations` call and no direct
`VisibleReader.GetEntity` remain in the relation gate.
- [ ] The interface is expressed without store types; `just arch-lint` clean
(`internal/validation` still does not import `internal/store`).
- [ ] Behaviour of the 14 shipped gates is unchanged, pinned by their
existing tests, at the seam-only commit AND at the end.
- [ ] `direction: incoming` counts edges arriving at the entity; a test fails
if the far endpoint is taken from the wrong end of the relation.
- [ ] Omitting `direction` behaves exactly as today.
- [ ] `target_type:` restricts counting to that type; a relation reaching
several types is narrowed correctly.
- [ ] `where` resolves against `target_type`'s definition when set, and an
unknown property is a LOAD error rather than a per-entity check-time error.
- [ ] An invalid `direction` value is a load error.
- [ ] A `target_type` the keyed relation cannot reach is a load error.
- [ ] The fail-closed semantics (per-bound unevaluable-target handling,
unrunnable constraint reported not swallowed) are preserved with tests.
- [ ] Face behaviour is unchanged from the shipped gate, pinned by a test.
- [ ] The two atlas `procedure` rules are expressible at `severity: error`.
- [ ] `just ci` green.

## Risks

- **Reading the wrong endpoint.** Mitigated by putting the endpoint choice
inside the adapter so the evaluator never sees an edge to pick from.
- **Silent no-match from an impossible `target_type`.** Guarded by the
load-time check; without it a typo produces a rule that passes forever.
- **Regressing the shipped gates while moving them onto the seam.** Mitigated
by doing the move as a separate, behaviour-preserving step with the existing
tests as the gate.
- **Scope creep into worlds/faces/batching.** All three are explicitly out of
scope and each has a criterion pinning "unchanged" rather than "improved".

## Context

Follow-up to TKT-R8QEU (done, PR #1571), which deliberately shipped the
outgoing-only, property-only form. Neither restriction was recorded as a known
limitation there — both surfaced from the atlas ISMS use case afterwards.

The originally-requested spelling was an expression form,
`count_relations(entity, 'gaat_over', {direction='incoming', ...})`. Not
proposed: the declarative block already shipped and covers the same ground, and
two syntaxes for one job is worse than one slightly wider block.

Precedent for the seam: `acl.Graph` (`internal/acl/graph.go:38`) — consumer-
declared, store-backed at the wiring site, with a null implementation for tests.
