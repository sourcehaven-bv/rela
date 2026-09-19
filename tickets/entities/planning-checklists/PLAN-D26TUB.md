---
id: PLAN-D26TUB
type: planning-checklist
title: 'Planning: reverse_relation migration step: rewrite stored edges when a relation type''s direction is swapped'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN:
- A `reverse_relation: {type}` migration step that rewrites stored edges of one
  relation type, swapping `from` and `to` per edge.
- A `relation_endpoints_swapped` delta kind in `CompareShapes` that RECOGNISES
  the swap, so the pair of narrowings stops being two unrelated findings.
- `migrate gen` drafting the step for that delta instead of the current
  "no declarative step can fix this" comment.
- A `resolvingSteps` entry so a file spanning the delta without the step is
  refused, matching how `faces_introduced` is enforced.
- Docs: the step table and a section in the data-migration guide.

OUT:
- Reversing `scope: content` relation types. REFUSED, not deferred — see Risks.
- Renaming as part of the same step (compose with `rename_relation_type`).
- Reversing a SUBSET of edges. The step is per relation type, matching every
  other step's granularity.
- Swapping the `inverse:` label, the cardinality bounds or `orderable:` in
  schema.yaml. Those are the operator's schema edit; the step reads them.
- Self-referential types, and more generally any type whose from-list and
  to-list share an entity type. REFUSED: the endpoint-type test that makes the
  step idempotent cannot decide which way such an edge points. See Idempotence.

**Acceptance Criteria:**

1. Reversing a relation type rewrites every stored edge, preserving properties
   and body content. Test: seed 3 edges with properties and content, apply,
   assert each exists reversed with both carried across and the original gone.
2. Re-running is a no-op — it does NOT flip the edges back. Test: apply twice,
   assert the second run affects 0 AND every edge still points the new way.
   (Asserting only the count would pass for a step that reversed everything a
   second time, since the count is the same.)
3. A `scope: content` relation type is refused before any write, naming the
   type and why. Refused at RUN time, not parse time — `Scope` is not in the
   shape projection (see Approach). Test: a dry-run `Runner.Run` over such a
   type returns an error and writes nothing.
4. A symmetric relation type is refused at parse time as a no-op that would
   churn every edge for nothing. Test: as above.
5. Swapping `from:`/`to:` in schema.yaml raises ONE
   `relation_endpoints_swapped` delta, not two `relation_endpoint_narrowed`.
   Test: `CompareShapes` over two metamodels.
6. `migrate gen` drafts a live `reverse_relation` step for that delta.
   Test: generate and assert the step is present and uncommented.
7. A file spanning the delta with no step is refused. Test: `ParseFile`.
8. Cardinality bounds that were NOT swapped alongside the endpoints are
   reported. Test: swap endpoints only, assert the draft carries the warning.
9. An edge whose endpoints do not typecheck after reversal is reported and
   left. Test: an edge whose `to` entity is not of a type the new `from`
   accepts.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: the option space is small
      and bounded by existing precedent — `rename_relation_type` already solved
      the identity-triple rewrite, so the question is which guardrails apply,
      not which architecture)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- `renameRelationTypeStep` (`internal/datamigration/steps.go:549-601`) is the
  direct precedent and near-identical in shape: a relation's type is part of its
  identity triple, so it recreates-then-deletes per relation, captures the
  delete synchronously for history, and tolerates `ErrConflict` on the create so
  a crashed run converges. Reversal has the same structure with `From`/`To`
  exchanged instead of `Type`.
- `collectRelations` (`steps.go:1104+`) already gathers every relation of a type.
- `migrate_face`'s `resolvingSteps` entry (`file.go:137`) is the precedent for
  a delta the classifier can DEMAND and a step must DELIVER.
- No external library applies: this is a store rewrite against rela's own
  identity model.

**Prior art checked and deliberately NOT followed:** `RenameEntity`
(`internal/store/store.go:428`) re-keys atomically and updates referencing
relations in one operation. There is no relation equivalent, and adding one is
out of scope — it would be a new `store.Store` method every backend must
implement and conformance-test.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. `reverseRelationStep{Type string}` in `steps.go`, registered in `parseStep`.
2. `Validate(from, to)`: the type must exist in both projections; refuse when
   `Symmetric` is true in either; refuse when the endpoints are NOT actually
   swapped (a guard against a hand-written step that would silently corrupt).
3. `Run`: `collectRelations(ctx, x.Store, s.Type)`, then a PRE-FLIGHT pass over
   the whole collected set (see "Pre-flight" below), then per edge — but ONLY
   for edges that still point the OLD way. See "Idempotence" below; this is the
   part that is not like `rename_relation_type`. For each such edge:
   `CreateRelation(ctx, r.To, s.Type, r.From, &store.RelationData{...})`,
   `x.captureRelationDelete`, `DeleteRelation`. Tolerate `ErrConflict` on
   create exactly as `rename_relation_type` does.

**Pre-flight: collisions and unreversible tails must be detected BEFORE any
write, and must be visible in dry-run.** Three cases make the naive
create-then-delete loop destructive. All three were verified empirically against
a memstore, not reasoned about:

- **A self-edge** (`A --t--> A`) reverses to a byte-identical triple, so the
  create returns `ErrConflict` against the very edge the delete then removes.
  Probe result: **0 survivors — the edge is destroyed.**
- **Both directions present** (`A --t--> B` and `B --t--> A`) makes
  `ErrConflict` ambiguous between "a prior crashed run already created this" and
  "a legitimate distinct edge exists here". Probe result: **2 edges became 1,
  and the survivor `A→B` carried the OTHER edge's properties (`w=BA`)** — silent
  corruption, not merely loss. On fs/memory there is no version capture at all
  (`newCapturer` returns nil when `Versions` is nil, `run.go:307`), so the
  original is unrecoverable.
- **A state-tailed edge** (`FromFace != ""`) has no reversed representation at
  all, because heads have no face slot.

So `rename_relation_type`'s `ErrConflict` tolerance (`steps.go:542`) must NOT be
copied. It is sound there only because the destination triple uses a type name
nothing else writes; here source and destination share one namespace.

`Run` therefore does, before any write and regardless of `x.Apply`:

1. Refuse if any collected edge has `r.FromFace != ""`, naming the edge. This is
   the load-bearing guard for content-scoped data — a DATA-level check, not the
   schema-level `scope:` check first planned, because `scope:` can be stale
   relative to what is stored (it may have been flipped to `identity` while
   state-tailed rows remain) and the migration subsystem exists precisely
   because the schema is not a reliable description of the store.
2. Skip self-edges (`r.From == r.To`) outright: reversal is identity. They do
   not count toward `Affected`.
3. Build the set of existing relation keys, compute every reversed key, and
   refuse if a reversed key collides with an edge that is not itself in the
   reversal set — naming both edges.

Because all three run before the `x.Apply` branch, a dry-run reports the refusal
rather than a clean count for a run that would destroy data. That fidelity is
what makes "the review is the safety mechanism" a real mitigation rather than an
assumed one.

Additionally, the per-edge write must carry the tail explicitly
(`store.RelationData{..., FromFace: r.FromFace}`) and delete via
`DeleteRelationState(ctx, r.From, r.FromFace, s.Type, r.To)` rather than
`DeleteRelation`. Guard 1 makes both moot today by refusing every non-zero tail,
but writing them the narrow way means a future relaxation cannot silently
reintroduce the defect `DeleteRelationState` was added to prevent
(`store.go:519-537`).

**Idempotence needs a directional test, and the naive design does not have
one.** Every other step has a trigger that fires only on untransformed data —
`rename_property` fires only where the old key is present, `map_values` only on
old values, `migrate_face` only on zero-coordinate rows. A reversal has no such
marker: `collectRelations` returns every edge of the type regardless of
orientation (`steps.go:1104+` → `store.RelationQuery{Type}`), and a reversed
edge is byte-identical in shape to an unreversed one.

Verified empirically on a memstore before writing this: reversing
`TSK-1 --assigned-to--> PER-1` yields `PER-1 --assigned-to--> TSK-1`, and the
next `collectRelations` returns that edge with nothing to distinguish it. A
naive step therefore **oscillates** — run twice and the data is back where it
started — which violates the `Step` contract stated at `steps.go:21-24`, where
re-running is the documented crash-recovery mechanism. A crash mid-file would
be unrecoverable by the system's own recovery procedure.

The test that IS available is the endpoint entity TYPES. An edge is
already-reversed when its `from` entity's type is in the TO-shape's `from:` list
(equivalently: its old orientation is no longer admissible). So `Run` resolves
each endpoint's type via `x.Store.GetEntity` and moves only edges whose current
`from` type is in the FROM-shape's `from:` list and whose `to` type is in the
FROM-shape's `to:` list. Consequences to accept and test:

- It costs two entity reads per edge. Acceptable: migrations are offline,
  operator-invoked and already do a full graph scan per step. If it is not,
  batch through `ListEntityHeaders` (TKT-1U8XYN's header path) rather than
  caching ad hoc.
- **A symmetric-endpoint type cannot be made idempotent this way.** When the
  from-list and to-list share a type (`from: [task] → to: [task, note]`), an
  edge between two `task`s satisfies both orientations and the test cannot
  answer. The step must REFUSE such a type rather than silently oscillate on
  the ambiguous subset. This subsumes the self-referential case listed under
  OUT, and makes it a refusal rather than merely undraftable.
- An edge whose endpoints typecheck under NEITHER orientation is left and
  reported, as planned in criterion 9.
4. `compareRelationShapes`: before the per-side `compareEndpointList` calls,
   test for a swap (`slices.Equal(fromRel.From, toRel.To) &&
   slices.Equal(fromRel.To, toRel.From)`, both non-empty and not already equal)
   and emit one `relation_endpoints_swapped` at `TierMigration`, skipping the
   two per-side comparisons for that relation.
5. `resolvingSteps["relation_endpoints_swapped"] = {"reverse_relation"}`.

   **`validateDeltasResolved` must be generalized first, or the entry silently
   enforces nothing.** As written (`file.go:180-189`) it builds its `present`
   map by type-switching on `*migrateFaceStep` alone and keying by `cf.Entity`
   — a bare entity name. A relation delta's subject is `rel:<name>`
   (`shapecompare.go:250`), and a `reverse_relation` step is not a
   `*migrateFaceStep`, so the lookup `present[kind][d.Subject]` would miss on
   both counts and the "file spans the delta but has no step" check would pass
   for every file. That failure is invisible: the guard test
   `TestResolvingSteps_CoversEveryMigrationDeltaKind` only checks that the KIND
   is listed, not that the enforcement fires.

   So: give `Step` an optional subject-reporting seam (a small interface like
   `resolvedSubject() string` that `migrateFaceStep` and `reverseRelationStep`
   implement, returning `cf.Entity` and `"rel:"+s.Type` respectively), and
   type-switch on that instead of the concrete type. Acceptance criterion 7
   must assert the refusal actually fires for the relation delta, not merely
   that the map has an entry.
6. `draftActiveStep`: emit a live `reverse_relation` step, plus a comment when
   the cardinality bounds were not swapped with the endpoints.

**The scope: content refusal cannot be validated from the projections.**
`ShapeProjection.RelationShape` (`shapeprojection.go:82-92`) carries `From`,
`To`, `Symmetric`, the four bounds, `Content` and `Properties` — NOT `Scope` or
`Orderable`. So `Validate(from, to)` cannot see whether the type is
content-scoped. Two options:

  (a) Add `Scope` to `ShapeProjection`. Rejected: it changes the shape hash for
      every existing project, forcing a re-baseline, and `scope:` does not
      affect whether a STORED edge conforms — which is the projection's stated
      membership rule.
  (b) Check at RUN time against the live metamodel, which `Exec` does not carry
      today. `Runner` has `deps.Meta`, so `Exec` gains a `Meta` field and the
      step refuses in `Run` rather than `Validate`.

      `Exec` has two construction sites: `run.go:131` (the runner, which has
      `deps.Meta` directly) and `gc.go:189` (the GC, which holds
      `Meta func() *metamodel.Metamodel` — hot-reload aware, `gc.go:39`). The
      GC only ever runs drop steps, so it may leave the new field nil; the
      reverse step must then fail loudly rather than skip its check, since a
      silently-skipped scope check is the lossy path this guards.

Taking (b). The cost is that the refusal is a run-time rather than parse-time
error, so it surfaces on `migrate data` (dry-run included) rather than on
`ParseFile`. That is acceptable because dry-run is the default and reports it
before any write. Note the same limitation applies to `_order_out`/`_order_in`:
`Orderable` is not in the projection either, so the order-property swap is also
a run-time concern.

**Files to modify:**
- `internal/datamigration/steps.go` — the step, registration
- `internal/datamigration/run.go` — `Exec.Meta`
- `internal/datamigration/file.go` — `resolvingSteps` entry
- `internal/datamigration/generate.go` — drafting
- `internal/metamodel/shapecompare.go` — the delta kind + `migrationDeltaKinds`
- `docs-project/entities/guides/GUIDE-data-migration.md` — step table + section
- tests alongside each

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- The migration file (`migrations/*.yaml`), operator-authored and committed.
  Trust boundary is the operator shell, identical to every other step. The step
  takes ONE field, a relation type NAME, validated to exist in both embedded
  projections — an allowlist by construction, not a pattern match. A typo is a
  parse error, never a silent no-op.
- No user input, no network, no file paths, no command execution.

**Security-Sensitive Operations:**

- Raw store writes bypassing entitymanager (no ACL, no validation, no
  automations). This is the sanctioned data-migration exception already
  documented in CLAUDE.md, and it inherits the runner's guarantees unchanged:
  the migration lock, `store.WithAttribution` naming the tool, one audit record
  per file, and synchronous pre-delete version capture.
- Relation versioning: the delete is captured via `x.captureRelationDelete`,
  the same path `rename_relation_type` and the cascade use. Skipping it would
  lose the pre-reversal state on the database backends, where the sweep cannot
  reconstruct a deleted row.
- Errors name relation types and entity ids, which are not secrets
  (CLAUDE.md: entity EXISTENCE is secret on the read path; the operator shell
  is not that boundary, and every other step already names ids).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** one per acceptance criterion above, in
`internal/datamigration/reverserelation_test.go` and
`internal/metamodel/shapecompare_test.go`, table-driven with `t.Run` subtests.

Integration: a `Runner.Run` test over a seeded memstore asserting the whole
file applies and the marker advances, not just the step in isolation.

**Edge Cases:**

- A relation type with zero stored edges: applies cleanly, affects 0.
- A self-edge (`A --type--> A`): reversal is identity, so create sees
  `ErrConflict` on the edge that is about to be deleted — and the delete would
  then destroy it. Create-then-delete is NOT automatically safe here. Moot in
  practice once shared from/to types are refused (a self-edge requires them),
  but the guard is cheap and the refusal is the thing to test.
- A second run over already-reversed data: must affect 0 AND leave the
  direction alone. This is the oscillation case; see Idempotence.
- Both directions already present (`A→B` and `B→A`): reversing produces a
  collision on an edge that legitimately exists. Refuse, naming both.
- An edge whose reversed endpoints do not typecheck under the new schema
  (the `to` entity is not of a type the new `from` accepts): report and leave,
  matching `convert`'s "unconvertible values are left in place and reported".
- Relation properties including the managed `_order_out`/`_order_in`.
- A relation type with `content: true` carrying a markdown body.

**Negative Tests:**

- `scope: content` type → refused at run, naming the type and why (no `ToFace`).
- `symmetric: true` type → refused at parse.
- Endpoints not actually swapped → refused at parse.
- Unknown relation type → parse error from the existing target validation.
- A file spanning `relation_endpoints_swapped` with no step → `ParseFile` error.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

1. **Content-scoped edges are unrepresentable reversed, and silently lossy if
   missed.** `entity.Relation` has `FromFace` and deliberately no `ToFace`
   (`entity.go:260-266`): heads are entity-level by construction, which is what
   makes cross-world dangling references inexpressible. Reversing
   `(A@draft) --t--> B` has nowhere to put `draft`. Worse, the tail face is part
   of the key (`entity.go:302-308`), so two edges differing only in tail face
   are two relations that a reversal collapses onto one key — a silent merge.
   *Mitigation:* refuse the whole relation type at run time, before any write.
   Effort `l` rather than `m` largely because `Exec` must learn the metamodel
   to do this.

2. **Version lineage breaks per edge.** Endpoints are the address
   (`UpdateRelation` mutates only `data`), so this is delete+create and each
   edge starts a fresh lineage on the database backends.
   *Mitigation:* none available; it is inherent, identical to
   `rename_relation_type`, and must be stated in the step's doc and the guide
   rather than discovered.

3. **A partial swap is ambiguous.** Endpoint lists that overlap rather than
   exchange cleanly (`from: [a,b] → [b,a]`) are not a reversal.
   *Mitigation:* the delta fires only on an exact exchange of non-empty,
   non-equal lists; anything else stays two narrowings, which is the current
   behaviour and is safe.

4. **The generator could draft a step the operator did not intend**, since a
   deliberate double-narrowing that happens to look like a swap is
   indistinguishable from one. *Mitigation:* accepted. The draft is reviewed
   before it applies — "the review is the safety mechanism" — and the step is a
   whole-type rewrite that a reader will not miss.

**Effort:** l

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] `docs-project/entities/guides/GUIDE-data-migration.md` — the step table
      gains a row; a new section covers the reversal, the lineage cost, and the
      content-scope refusal. `docs/data-migration.md` regenerates from it.
- [ ] docs/metamodel.md — no metamodel feature is added; `from:`/`to:` already
      exist and the operator edits them by hand.
- [ ] CLAUDE.md — no new pattern.

## Design Review

- [ ] Run `/design-review` before starting implementation
- [ ] All critical/significant findings addressed in plan

**Design Review Findings:**
