---
id: BUGA-WD1DG8
type: bug-analysis-checklist
title: 'Analysis: Bounded relation replace skipped the audit log on a type mismatch'
started: "2026-10-22"
completed: "2026-10-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally
- [x] Minimal reproduction steps documented
- [x] Environment/conditions noted

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

## Notes

Reproduced by
`TestPatchRelations_DisallowedTypeFallbackKeepsAllowedCreateOnManager` on
develop: a full linkage on a bounded relation (`pair`, `max_outgoing: 2`) naming
a disallowed `tag` and an allowed `category` returns 200 and stores both edges,
but the audit sink holds only the `C-1` create. The test asserted that as
intended behaviour. Any backend; NopACL.

Fix: `writeBoundedOps` sets `TolerateTypeMismatch` on every create it passes to
`ReplaceRelations`, which already forwards each create's options to
`prepareRelationCreate`. The replace then runs in one manager call: one Tx,
audit records for every create and remove, relation template and managed order.
`writeBoundedFallback`, `splitDisallowed` and the exported
`entitymanager.CheckRelationCapacity` (only used by the fallback) are removed.

Related areas: `writeCreateRelation` already passes the flag (BUG-X1479P);
`writeUpdateRelation` never validates the allowlist; CalDAV's `ReplaceRelations`
call does not tolerate a mismatch and returns the error. `grep store.Tx
internal/dataentry` shows no other write around the manager. TKT-9DINM3 (POST
relations strict on mismatch) stays a separate ticket.
