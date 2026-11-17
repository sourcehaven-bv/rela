---
id: REV-IDBKR7E
type: review-checklist
title: 'Review: Mutable record types'
started: "2026-11-17"
completed: "2026-11-17"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (N/A locally: ran the affected packages instead (records, filerecords, sqliterecords, sqlitedb, metamodel, audit, appbuild with and without -tags sqlite, dataentry); CI runs the full suite and the postgres job)
- [x] Lint clean (`just lint`): golangci-lint on the changed packages, `just arch-lint`
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A locally: new code is covered by new tests in records, dataentry and appbuild; CI enforces the floors)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-Y361655, RR-1R7HZC9, RR-XOONQT8 (significant,
addressed); RR-ZALB0ZD, RR-UDZ2GZ2, RR-4STKX8L, RR-SP9H96N, RR-1OQSKB4,
RR-X7EFU9D (minor, addressed); RR-9HY2CEG, RR-32SFWF7, RR-8E8N769 (nit,
wont-fix). The rela-security-reviewer also ran; its two minor findings are
RR-4STKX8L and RR-UDZ2GZ2.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** 1 PASS (TestMetamodel_LedgerCannotBeMutable). 2 PASS
(TestUpdate_ChangesRowAndAudits, TestRecords_MutableUpdateDeleteSQLite). 3 PASS
(TestDelete_RemovesRowAndAudits). 4 PASS (TestChange_Refuses,
TestChange_AuditsDenial, TestRecordsHandler_ChangeRow). 5 PASS (recordstest
Replace, ReplaceMovesTS, Delete, ConcurrentReplace on file and sqlite; postgres
in CI).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-9HVPY4A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the PR is opened after the ticket is done; see TKT-UFV01M)
