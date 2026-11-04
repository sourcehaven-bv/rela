---
id: PLAN-R4M797S
type: planning-checklist
title: 'Planning: Atomic batch engine: entitymanager.CreateBatch'
started: "2026-11-04"
completed: "2026-11-04"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope: `entitymanager.CreateBatch(ctx, m, Batch)`, a package function that
creates N entities and M relations in one `store.Store.Tx`. Ends of a relation
are a batch index or an existing id (with an optional tail face). The checks run
before the Tx, the writes inside it, and audit, alias notifications and
automations after commit.

Out of scope: relation properties, updates and deletes in a batch, automations
inside the Tx (TKT-41YLNDP), Lua `rela.tx` (TKT-I9QKZGH), the HTTP surface
(TKT-BO8CYG9) and the capability flag (TKT-ZMPDZ3R). The engine does not refuse
fs/memory stores itself; it documents their kept-writes behaviour and the caller
(the endpoint) refuses them with 501.

**Acceptance Criteria:**

1. A batch of 3 entities and 3 edges (local-local, local-existing,
existing-local) commits; every edge resolves to the generated ids.
2. A failure in the last operation leaves, on sqlite and postgres, no rows, no
audit records and no store events.
3. On fs/memory the same failure returns a `BatchKeptError` listing what
stayed, and every kept row is audited.
4. A refused check (type-disallowed edge) writes nothing on any store and
names the operation (`BatchOpError{Kind, Index}`).
5. An existing end that is missing or hidden from the caller is refused the
same way (`ErrEntityNotFound`).
6. Shape refusals (empty, over 100 operations, nil entity, index out of range,
head with a face) are `ErrBatchInvalid` and audit nothing; exactly 100
operations is allowed.
7. Audit order after commit: every create-entity record, then every
create-relation record, then follow-ups.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-QKREXZZ, decision DEC-ODU65KR.

**Existing Solutions:**

- No library: the transaction seam is `store.Store.Tx` (DEC-8UIL0) and the
rules forbid any abstraction above it.
- `CreateLinked` (internal/entitymanager/linked.go) is the one-entity,
one-edge version: `prepareCreate` and `prepareRelationFrom` before the Tx,
`createCore` and `writeLinkedRelation` on the view, `finishCreate` after, and
`finishKept` on stores without rollback. CreateBatch generalizes it.
- Reference: JSON:API atomic operations and the Microsoft Graph `$batch`
(surveyed in RES-QKREXZZ).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. `checkBatchShape` refuses an unrunnable batch before any check that could
audit a denial.
2. `prepareCreate` per entity, in order. `prepareBatchRelation` per relation:
a local source is judged from its planned family (type, face, custom id); an
existing source goes through `lookupSource`, which treats a hidden source as
missing; a local target is passed as the `created` family so it is never looked
up; an existing target goes through the read gate in `prepareRelationFrom`.
3. In the Tx, `deps.Store = view`: `createCore` per entity (ids minted under
the store's write lock, the view sees earlier batch rows so sequential ids and
`unique:` scans include them), then fill the local ids into each key and
`writeLinkedRelation` on the view (cardinality and owning counted on the view).
Nothing touches `m.deps.Store` inside the callback.
4. After commit, `finish` records all audits first, then runs
`createFollowUps` per entity. Errors go to `FollowUpErrs`.
5. On failure, `failed` re-reads each written row from the outer store; any
that stayed (fs/memory) is audited and followed up and reported as
`BatchKeptError`.
6. `finishCreate` is split into `recordCreate` and `createFollowUps` so the
batch can order them; single creates are unchanged.

Alternatives rejected: a Manager method (Manager is at its plimsoll method cap);
running checks inside the Tx (holds the global lock during ACL work and risks
audited denials for rolled-back work); a compensating-delete rollback for
fs/memory (not atomic under crash, rejected in RES-QKREXZZ).

**Files to modify:**

- internal/entitymanager/batch.go (new)
- internal/entitymanager/batch_test.go (new)
- internal/entitymanager/manager.go (split `finishCreate`)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

The batch comes from the caller (later: the HTTP body). Each entity and relation
goes through the same validation as a single create: schema type and properties,
face, custom id rules, relation type tuple. Indexes are checked against the
batch length. Nothing in the batch selects a backend or a path.

**Security-Sensitive Operations (threat model, POLICY-015 §3 / PROCEDURE-j5mq
step 1):**

- ACL per operation: each entity create runs the create ACL and the field
gate as the caller; each relation runs the relation ACL as the caller. A batch
grants nothing a sequence of single creates would not. The relation ACL runs
before a local source has an id; this is sound because the relation decision
reads only the source type and the caller's global roles (pinned by
`acl.TestRelationWrite_DecisionIgnoresSourceID`).
- Hidden endpoints: an existing end the caller cannot read is refused with the
same `ErrEntityNotFound` as a missing id (lookupSource and
requireVisibleTarget). No message names whether it exists.
- Limits: 100 operations, so the Tx holds the write lock for a bounded time.
The body limit is enforced at the HTTP edge.
- Audit: no audit record for a rolled-back batch; one record per row after a
commit; kept rows on fs/memory are audited so no row exists without a record.
Denials during prepare are audited as single creates audit them.
- Id minting under the lock: generated ids are minted inside the Tx on the
view, so two concurrent batches cannot mint the same sequential id (pg advisory
lock, sqlite BEGIN IMMEDIATE, fs/mem write mutex).
- Partial commit per backend: postgres and sqlite roll back (native
transaction, events buffered to commit); fs and memory do not, which is why the
endpoint answers 501 there and the engine reports `BatchKeptError`.
- External I/O: none in the Tx; automations run after commit
(`.claude/rules/jobs.md`).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

All in internal/entitymanager/batch_test.go over `concBackends` (memstore,
fsstore, sqlite with `-tags sqlite`, postgres with `-tags postgres` and a
schema-pinned DSN):

1. `TestCreateBatch_CommitsTogether` (AC 1, 7)
2. `TestCreateBatch_MidBatchFailureRollsBack` (AC 2, 3), with
`TestCreateBatch_RollbackThenCommitDeliversEvents` as the control for "no
events"
3. `TestCreateBatch_RefusalWritesNothing` (AC 4)
4. `TestCreateBatch_MissingEndpoint` (AC 5; the hidden case is pinned at the
HTTP layer with an ACL)
5. `TestCreateBatch_Shape`, `TestCreateBatch_ExactlyMaxOperations` (AC 6)
6. `TestCreateBatch_UniqueWithinBatch`: two persons with the same unique
email in one batch roll back.

**Edge Cases:**

- Empty batch, 100 and 101 operations, nil entity, negative or out-of-range
index.
- Duplicate edge inside the batch (fails in the Tx, not in prepare).
- Duplicate custom id or unique value inside the batch (fails in the Tx).
- Edge from an existing faced entity (tail face).
- Concurrent batches: ids minted under the lock.

**Negative Tests:** the shape, refusal, missing-end and rollback tests above.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- A check that passes before the Tx but fails inside it (duplicate edge,
unique clash, cardinality). Mitigation: the Tx rolls back on sqlite/pg; pinned
by the rollback test.
- Splitting `finishCreate` changes the single-create path. Mitigation: pure
extraction; the existing entitymanager tests cover it.
- Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** godoc on CreateBatch. User-facing docs belong to
TKT-BO8CYG9.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-T1M860X (critical), RR-RLSQHTM, RR-3LT4WBL
(significant), RR-734XHMA, RR-RVPDJ0L, RR-SE3ULLG, RR-2EGZ18C (minor). All
critical and significant findings are addressed; RR-2EGZ18C is deferred to a
backlog ticket.
