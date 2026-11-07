---
id: PLAN-LXQG6QO
type: planning-checklist
title: 'Planning: Erase deleted-entity state from automation job payloads'
started: "2026-11-06"
completed: "2026-11-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: the deleted-entity state that queued `on.deleted` jobs carry;
erasure on job end and on purge (soft-delete purge, `rela history-purge`);
retention of finished and dead rows in the postgres job tables. Out: per-face
soft delete (R12, parked); foreground delivery, which stores nothing.

**Acceptance Criteria:**
1. The job payload never holds the deleted state in clear.
2. After a job completes, is dropped, or fails its final attempt, its state
cannot be read; a retryable failure keeps it.
3. After a purge of the entity, pending jobs cannot read its state and end.
4. The postgres queue deletes finished and dead rows after the retention period
and keeps pending ones.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: small follow-up; options listed in the CISO finding)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: crypto-shredding is a standard pattern)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** Standard library AES-GCM; `state.KV` as used by the
trigger tokens in `internal/appbuild/automationjobs.go`; the optional-notifier
pattern of `entitymanager.DeleteNotifier`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Seal the state per job with AES-256-GCM; the key lives
in `state.KV` under the hash of the entity id with an index of handles. Erase
the key when the job ends and on purge. Sweep finished and dead neoq rows after
24 hours.

Rejected: keeping the plain state in `state.KV` (orphans after a crash or a
dispatcher drop would be plain data on disk, and KV cannot list keys for a
sweep); scrubbing the job tables only (a memory queue in another process cannot
be reached by `rela history-purge`).

**Files to modify:**
`internal/appbuild/{automationjobs,deletedstate,aliasfanout}.go`,
`internal/entitymanager/{alias_hook,softdelete}.go`, `internal/jobs/pgqueue.go`,
`internal/cli/history_purge.go`, three guides in `docs-project/`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** The handle comes from a job payload and reaches
a KV key, so it must be 32 hex characters. The entity id is hashed before it
reaches a key.

**Security-Sensitive Operations:** AES-256-GCM with a random nonce; the AAD
binds the face ref and handle so one job's ciphertext cannot be opened as
another face.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
1. `TestDeletedState_PayloadHoldsNoClearState`
2. `TestDeletedState_ErasedWhenJobEnds`
3. `TestDeletedState_ErasedOnPurge`, `TestPurgeNotifier`
4. `TestPostgresQueue_RetentionSweep` (schema-pinned DSN)

**Edge Cases:** Refused enqueue (`TestDeletedState_EnqueueFailureErases`); purge
of one entity keeps another's state.

**Negative Tests:** Tampered ciphertext, another face, missing sealed text,
malformed handle (`TestAutomationJobs_DeletedPayload`).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Cross-process index races can lose an index entry; a purge then
misses that key, but the job still erases it at its end and the sweep removes
the row. Effort: s.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/metamodel.md
- [x] docs/cli-reference.md
- [x] docs/postgres-backend.md

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: approach weighed in the CISO finding's options; a code and security review runs at review)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** none
