---
id: PLAN-02R4KL
type: planning-checklist
title: 'Planning: Version snapshots do not record which face they captured'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: read the stored `face` column back into `VersionMeta.Face` on
pgstore and sqlitestore, and use it on restore. Out: capture changes (the face
is already stored since migration 0012), lineage walk changes.

**Acceptance Criteria:**
1. A snapshot read at `(id, face)` reports that face in its meta, on both
database backends (storetest `VersionsRecordTheirFace`).
2. Restoring a deleted face recreates it at the same id and face
(`TestFacedHistory_*` recreate test, e2e `faces-history.spec.ts`).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small read-path change)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: internal read-path gap)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal read-path gap)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** `entity_versions` already keys on `(entity_id, face,
...)`; the scan in `pgstore/version.go` and `sqlitestore/version.go` only had to
select `ev.face`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Add `Face entity.Face` to `store.VersionMeta`; scan it
in both backends; `serveHistoryVersion` sets the snapshot entity's face; restore
recreates a deleted face with `ApplyEntity` at its own id. Alternative rejected:
deriving the face from the request address only, which leaves snapshot JSON
without its coordinate.

**Files to modify:** `internal/store/store.go`,
`internal/store/{pgstore,sqlitestore}/version.go`,
`internal/store/storetest/version.go`, `internal/dataentry/history_*.go`,
`internal/cli/history.go`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** The face comes from storage, not from the
caller. The address is parsed by `entity.ParseRef`.

**Security-Sensitive Operations:** Restore of a deleted face is authorized as a
create on `type@face` by the entity manager.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** Storetest on both backends; dataentry handler tests; e2e on
postgres.

**Edge Cases:** Zero face on an unfaced type reports an empty face; two faces
with identical bytes stay distinct.

**Negative Tests:** A restore to a face the principal may not create is refused.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Widening the lineage walk across faces; mitigated by leaving the walk
unchanged and pinning face independence in storetest. Effort: s.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: docs are covered by BUG-4SYAA6 in the same PR)

**Documentation Impact:** Snapshot JSON gains `face`; documented with the
per-face history docs of BUG-4SYAA6 (cli-reference, content-states).

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: design fixed by the Stage 1 design doc, reviewed with TKT-2528AB)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** N/A
