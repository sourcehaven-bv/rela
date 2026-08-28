---
id: PLAN-KG8JBE
type: planning-checklist
title: 'Planning: Make filesystem migration stale-break acquisition atomic'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In scope: make filesystem stale-lock replacement one atomic critical
section under the existing exclusive `.break` marker and strengthen its
concurrency regression coverage. Out of scope: PostgreSQL/process locks,
distributed or network-filesystem guarantees, lock format changes, and timeout
policy changes.

**Acceptance Criteria:**
1. Two independent `fsLock` values racing a pre-seeded stale lock yield exactly
one successful acquisition under repeated race-detector execution.
2. A contender cannot inspect or replace the lock between stale-file removal
and publication of the winner's payload.
3. Live, missing, unparseable, abandoned-break, context-cancel, guarded-release,
and exclusive/release behavior remain passing.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: xs concurrency fix)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A; direct repair of the existing O_EXCL protocol.

**Existing Solutions:** `fsLock.acquireFile` already uses O_EXCL for both the
ownership file and a break marker. The weak point is releasing the break marker
after deleting the stale owner and only reacquiring it on the next loop
iteration. Standard lock replacement holds the serialization primitive across
delete-and-create; no new library is needed. Existing staleness and
guarded-release tests provide the compatibility envelope. The failure was
observed in PR #1450's unrelated Test job while its PostgreSQL and all other
gates passed.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Acquire the existing `.break` O_EXCL marker once, defer
its removal, attempt the owner-file create, and, when an existing owner is
stale, remove and retry the owner-file create while still holding that marker.
Return contention if replacement loses unexpectedly; preserve context/error
wrapping. This removes the unlock/relock gap and the two-iteration control flow.

Rejected: weakening or deleting the flaky test hides a real mutual-exclusion
contract; a package-global mutex would only mask the issue in one process and
would not repair the on-disk protocol; retries/sleeps add timing dependence.

**Files to modify:**
- `internal/datamigration/lock.go`
- `internal/datamigration/lock_test.go` only if deterministic assertions need extension

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** The only input is the existing local lock file.
Its JSON/PID validation remains unchanged: live PIDs block, dead or unparseable
records are stale, missing files are created exclusively.

**Security-Sensitive Operations:** Deletion and recreation of the migration
ownership file affect write serialization. Both remain protected by O_EXCL and
now occur under one held break marker. Logs expose only the local lock path, as
before.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** Run the focused concurrent test repeatedly under `-race`;
run the whole datamigration package; then project lint, architecture, coverage,
and CI gates. The existing test uses two independently constructed lock values
and therefore exercises the filesystem protocol rather than the per-instance
mutex.

**Edge Cases:** Stale present, missing owner, live owner, malformed payload,
abandoned break marker, cancellation, write failure, double/stale release, and
two contenders.

**Negative Tests:** Live ownership and concurrent break ownership return
`ErrLockHeld`; canceled contexts and filesystem failures retain wrapped errors.
No waiter blocks.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Risk is accidentally leaving `.break` behind or changing fail-fast
semantics; a defer guarantees cleanup and existing tests pin both abandoned
markers and contention. Effort: xs.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] ~~User-facing docs identified~~ (N/A: internal correctness fix, no behavior/API change)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: chore)

**Documentation Impact:**
- [x] N/A — internal locking correction; existing lock documentation remains accurate.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** No open findings. The key design constraint is to
hold the cross-process marker across stale removal and replacement; package-only
mutexes and timing retries were rejected because they do not establish that
invariant.
