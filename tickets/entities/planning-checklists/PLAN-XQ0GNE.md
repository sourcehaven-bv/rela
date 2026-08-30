---
id: PLAN-XQ0GNE
type: planning-checklist
title: 'Planning: Audit already-deleted relations when a cascade delete fails partway'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** when a cascade delete fails partway — an I/O error removing
relation R2 after R1 is already off disk — `fsstore.deleteEntity` returns `nil,
err`. `Manager.DeleteEntity` returns early on that error, so the relations that
**were** deleted get no audit record. The log does not reflect the real system
state. GitHub issue #929 (severity: low, POLICY-015 §4).

**Why the loss is real and not recoverable elsewhere.** fsstore's `Tx` is a
write mutex with *mutual exclusion only* — no rollback (CLAUDE.md, DEC-8UIL0).
So the already-removed relation files stay removed; the store's own comment says
so: *"Not transactional: a relation file removed before a later failure stays
removed."* The manager's audit loop reads `res.DeletedRelations`, and `res` is
nil on the error path, so nothing is emitted. The deletion happened and the log
denies it.

**Scope — IN:**

- `fsstore.deleteEntity` returns a **partial** `*store.DeleteResult` alongside the
error, naming the relations it did remove.
- `Manager.DeleteEntity` audits those before propagating the error.
- The `store.EntityWriter` contract documents that a non-nil result may accompany
a non-nil error, and that callers must treat it as "this much really happened".

**Scope — OUT:**

- Making fsstore's cascade delete atomic. That is the actual cure, and it is a
much larger change (DEC-8UIL0's Tx tier is deliberate); this ticket makes the
log honest about the non-atomicity, it does not remove it.
- pgstore / sqlitestore. Both are genuinely transactional, so a partial delete
cannot persist there — the new contract *permits* a partial result, it does not
require one, and those backends keep returning `nil` on error.
- The entity-file and attachment-dir failure paths (lines after the relation
loop). Those abort *after* all relations are gone, so the same treatment applies
and falls out of the same change — but the entity itself is demonstrably still
present, so it must NOT be audited as deleted.
- Any change to `Manager`'s fail-secure ordering. #899 established it; this
builds on it.

**Acceptance Criteria:**

1. **AC1** — when the relation loop fails on the Nth relation, the returned
`*DeleteResult` names exactly the N-1 relations already removed, and the error
is still returned unchanged.
2. **AC2** — `Manager.DeleteEntity` emits one `delete-relation` audit record per
relation in that partial result, carrying `triggered_by:
"cascade:delete-entity:<id>"` (the same label the success path uses), and then
returns the error.
3. **AC3** — **no `delete-entity` record is emitted** on the failure path. The
entity file is untouched when the relation loop aborts, so auditing its deletion
would be the opposite error: a log claiming something that did not happen.
4. **AC4** — the success path is byte-for-byte unchanged: same result, same audit
records, same ordering.
5. **AC5** — the `store.Store` contract states the partial-result rule, and
`storetest` does not begin requiring it of transactional backends.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — small, single-backend, and the shape is dictated by the
existing audit path.

**Existing Solutions / prior art:**

- The **success path is the template**: `Manager.DeleteEntity` already wraps ctx
with `cascade:delete-entity:<id>` and loops `res.DeletedRelations` emitting
`OpDeleteRelation`. The fix is to reach that same loop on the error path — not
to invent a second audit shape.
- **`cascadeHost.DeleteEntity`** does the identical thing for the
`if_exists: replace` path, and its comment explains why the label matters.
Whatever this ticket does must keep both consistent.
- **Returning a value with an error is already idiomatic here**:
`store.DeleteResult` exists precisely to report *what* a delete touched, and
`entity.CreateResult` carries `Warnings` alongside a successful create. Go's
convention that a non-nil error means "ignore the value" is the thing to
document an explicit exception to, in the interface godoc, rather than assume.
- **#899 / BUG-C20T** established the fail-secure ordering (relations first,
abort before the entity file). This issue is explicitly the residual edge case
that fix left, and the ordering is what makes AC3 correct — the entity really is
still there.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **fsstore** — accumulate successfully-removed relations in the loop. On
failure, return `&store.DeleteResult{DeletedRelations: done}, err`. Do **not**
populate `DeletedEntities`: the entity file is untouched at that point (AC3).
The in-memory index is still only mutated on success, so the store's own view
stays consistent with disk for the entity; the relation index entries for
already-deleted files are the pre-existing inconsistency this ticket documents
rather than fixes.
2. **Manager** — the delete runs inside `store.Tx`. On `txErr != nil`, audit any
relations the partial result names (under the same `cascade:delete-entity:<id>`
ctx) and then return the error. This must sit *outside* the Tx callback, since
the callback's return is what signals failure.
3. **Contract** — document on `store.EntityWriter.DeleteEntity` that a non-nil
`*DeleteResult` MAY accompany a non-nil error on a non-transactional backend,
listing only what genuinely happened; that transactional backends return nil;
and that a caller must not treat a partial result as success.

**Files to modify:** `internal/store/fsstore/entity.go`,
`internal/store/store.go` (contract godoc), `internal/entitymanager/manager.go`,
plus tests.

**Alternatives considered:**

1. *Audit inside fsstore.* Rejected outright — the store has no audit sink and
must not gain one. CLAUDE.md is explicit that audit is the entitymanager
write-path's concern; a store that audits would double-record every write that
already goes through the manager, and would record ones that deliberately bypass
it (import, sync, migration).
2. *Best-effort rollback: restore the removed relation files.* Rejected. The
content is in memory, so it is superficially possible — but a restore that
itself fails leaves a state neither the log nor the caller can describe, and it
converts a clean "this much happened" into a guess. Atomicity belongs in the Tx
tier (DEC-8UIL0), not in an ad-hoc undo.
3. *A typed `PartialDeleteError` carrying the relations.* Rejected as more
machinery for the same information: `DeleteResult` already exists, already means
"what this delete touched", and the manager already knows how to audit from it.
A new error type would need `errors.As` at the call site and a second audit
shape.
4. *Leave it; document the gap.* Rejected — the issue is that an audit log which
silently under-reports is worse than one that is merely incomplete-by-design,
and the fix is small.

**Dependencies:** none new.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** none new — this changes what is *reported* about
an operation the caller already authorized. No new input reaches the store.

**Security-Sensitive Operations:** this is an audit-log change, so:

- *Direction of error.* The change can only ADD records for deletions that
genuinely occurred. The failure mode to avoid is the opposite — recording a
deletion that did not happen — which is exactly what AC3 guards by refusing to
emit a `delete-entity` record when the entity file is untouched. An audit log
that over-reports is as broken as one that under-reports, and harder to detect.
- *Authorization is unchanged.* The ACL check happens in `Manager` before the
store is called; a partial failure does not re-enter that path. Nothing here
lets a caller delete something it could not already delete.
- *No content in the records.* `recordRelationAudit` writes the relation's
identity (from/type/to), the same as on the success path — no relation body or
properties.
- *The error itself* is a filesystem error naming a relation file path inside the
project. Same shape as the error already returned today; not widened.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** the whole ticket is a failure path, so the test has to
*induce* an I/O failure rather than wait for one. `fsstore` writes through
`storage.FS` / `RootedFS`, so the seam is a wrapper FS whose `Remove` fails for
one designated relation file and delegates everything else — that gives a
deterministic "fails on the Nth relation" without touching production code.

- **AC1** (`internal/store/fsstore`): entity with 3 relations; FS fails on the
2nd. Assert: error returned; result non-nil; `DeletedRelations` names exactly
the 1st; the 3rd is still on disk; the entity file is still on disk.
- **AC2/AC3** (`internal/entitymanager`, the integration test that matters):
same setup through `Manager.DeleteEntity` with a memory audit sink. Assert: one
`delete-relation` record for the 1st relation, carrying
`cascade:delete-entity:<id>`; **zero** `delete-entity` records; the error
propagates.
- **AC4**: the existing success-path tests are the assertion — they must pass
unchanged, and `git diff` must show no edits to them.
- **AC5**: `storetest` conformance still passes for memstore/fsstore; no new
requirement is added that a transactional backend would fail.

**Edge Cases:**

- Failure on the **first** relation → result names zero relations. Must return a
non-nil-but-empty result (or nil) *consistently*, and the manager must not emit
a stray record either way. Pick one and test it.
- Failure on the **entity file** after all relations are gone → all relations
audited, still no `delete-entity` record.
- `os.IsNotExist` on a relation file is deliberately NOT an error today (the file
was already gone). It must stay non-fatal and must **not** be counted as
deleted-by-us — auditing a deletion this call did not perform is the AC3 error
in miniature.
- `cascade=false` with no relations → path unaffected.
- The `if_exists: replace` route through `cascadeHost.DeleteEntity` hits the same
store call; verify it inherits the behaviour and its
`cascade:delete-entity:<id>` label is unchanged.

**Negative Tests:** the success path emits no partial-audit records; a
non-cascade delete is untouched; a transactional backend returning `nil, err`
still satisfies the contract.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Mitigation |
|---|---|
| A caller treats the partial result as success because Go convention says "error → ignore the value" | Document the exception on the interface godoc; keep `DeletedEntities` empty so a result that *looks* successful cannot be produced |
| Auditing a deletion that did not happen (over-reporting) | AC3; `os.IsNotExist` skips are not counted; tested |
| The fault-injection FS drifts from real `RootedFS` behaviour | Wrap the real one and fail a single designated path, rather than reimplementing |
| Scope creep into making the cascade atomic | Explicitly out of scope; DEC-8UIL0 owns the Tx tier |

**Effort:** s

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `store.EntityWriter.DeleteEntity` godoc — the partial-result contract.
This is the load-bearing doc change: it is the only place a future backend
author learns the rule.
- [x] `docs-project/entities/guides/GUIDE-audit-log.md` — the guide's "Known
gaps → Crash window" section discusses audit-vs-reality divergence; a partial
cascade delete is the same family and belongs beside it. Regenerate
`docs/audit-log.md` via `./scripts/generate-docs.sh` (never edit the generated
file).
- [x] ~~CLI reference~~ (N/A: no command or flag change)
- [x] ~~CLAUDE.md~~ (N/A: applies the existing audit rule rather than changing it)

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: skipped —
      and unlike the last ticket, this skip was the wrong call in a way worth
      recording precisely, because the same gap bit twice.)
- [x] All critical/significant findings addressed in plan — all 7 code-review
      findings addressed, none deferred. See the review checklist.

**Design Review Findings:** N/A as a separate pass. The honest post-mortem:

This plan reasoned carefully about the *relation* abort path and asserted, in
its Scope section, that the entity-file and attachment-dir paths "fall out of
the same change". They did not. The attachment path aborts **after** the entity
file is already gone, so returning `DeletedEntities: nil` there under-reported
the entity — #929's own failure mode, reproduced by the fix for #929
(RR-UE2XS7). The plan waved at those paths in one clause instead of tracing
them, and the implementation inherited the hand-wave as a comment asserting an
invariant that was false.

The plan also stated a Test Plan that mutation-tested the relation path only.
That is why the false comment survived to review: the untested path was exactly
the untraced one.

Two transferable lessons, both narrow:

1. **"Falls out of the same change" is a claim, not a scope reduction.** When a
   plan says several paths behave alike, trace each one — the cost is minutes
   and the failure mode is a confidently wrong comment.
2. **Mutation-test every path the change touches, not the representative one.**
   The relation path was verified and correct; the two paths that weren't
   verified were the two that were wrong.

Code-review findings: RR-UE2XS7 (critical); RR-JA8WRT, RR-M3SEHY, RR-17Y380
(significant); RR-UU2QP1, RR-TYG8OV (minor); RR-E6VJI5 (nit).
