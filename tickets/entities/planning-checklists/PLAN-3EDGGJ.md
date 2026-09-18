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

Read gating. The gate counts what the acting identity can SEE — the shipped docs
say so explicitly, and a gate is documented as not being a global invariant. The
adapter must therefore keep reading through `VisibleReader` rather than taking a
raw store handle; swapping to a raw handle would silently turn a
visibility-scoped gate into a global one and leak the existence of hidden
entities through violation counts. Criterion: visibility behaviour unchanged.

Fail-closed semantics must survive the move: an unevaluable target counts as
matching when `Max` is set and is skipped otherwise (each bound fails closed),
and a constraint that cannot run is reported as a `LoadError` rather than
silently passing. Losing either during the refactor converts a gate into a
no-op, which is the failure class this whole line of work exists to prevent.

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

Integration: the tickets project's own 14 gates ARE the integration test — `rela
validate` over `tickets/` must report identically before and after.

**Edge Cases:**

- Self-referencing relation (from-type == to-type): direction still
distinguishes the two ends; both must be counted correctly.
- Symmetric relation: confirm direction semantics are coherent, or reject.
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

**Effort:** m

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs/metamodel.md` — extend the shipped "Relation Cardinality
Validation" section with `direction:` and `target_type:`, including the note
that `where` is validated at load when `target_type` is set.
- [x] `docs-project/entities/guides/GUIDE-metamodel.md` — the mirrored copy
updated in the same commit (TKT-R8QEU updated both).
- [ ] N/A — CLAUDE.md: no new cross-cutting convention; the seam follows the
existing consumer-side-interface rule already documented there.

## Design Review

- [ ] Run `/design-review` before starting implementation
- [ ] All critical/significant findings addressed in plan

**Design Review Findings:** pending — see note below.
