---
id: REV-RR07NR
type: review-checklist
title: 'Review: fs-to-sqlite migration command'
started: "2026-10-06"
completed: "2026-10-06"
status: done
---

## Automated Checks

- [x] All tests pass (`just test`) — exit 0; also `-tags sqlite` for cli/fsimport/appbuild and `-race` for fsimport
- [x] Lint clean (`just lint`) — 0 issues
- [x] Comment lint gate clean (`just comment-lint`) — no unresolvable doc links
- [x] Coverage maintained (`just coverage-check`) — total 82.5%, all floors pass

`just arch-lint` and `just plimsoll` clean.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) — cranky-code-reviewer and rela-security-reviewer both ran
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed — RR-TEV88P, RR-3VTM0V (date normalization), RR-U9IYRL (symlinks)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** Code review: RR-TEV88P, RR-3VTM0V, RR-59SCDU, RR-QVBSEY,
RR-Y84F1C, RR-BXTGO1, RR-MR1YPC, RR-EMA9J8, RR-BR4WVK, RR-ZKCLN3, RR-VY26F2,
RR-0T7VPY, RR-1LIQ2G, RR-5Y1BPF, RR-2GAESG (wont-fix), RR-2J7JV0, RR-OR6LOZ,
RR-3Q6KTJ (deferred), RR-OYGZAC (wont-fix). Security review: RR-U9IYRL,
RR-G2BPUN, RR-8I4DY1, RR-LYIH02. The rest are design-review responses. None are
open.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. PASS — `TestRun_CopiesEverything`, `TestDBImportFS_SQLite` (faces, datetime, relation property, attachment, comment, state key, search; verify re-reads the renamed target). Verification compares normalized rows, not `canonical.Hash*` (deviation recorded in IMPL-MQ0ION).
2. PASS — source snapshot equality in the run tests; `TestDBImportFS_SQLite` compares every source file byte for byte.
3. PASS — `TestRun_RefusesEncryptedContent`; unreadable attachments are now collected too.
4. PASS — `TestRun_RefusesBadPaths`; a target appearing mid-run is refused (`TestRun_TargetAppearsDuringImport`).
5. PASS — `TestRun_BackendFailureLeavesNothing`, `TestRun_FailsWhenTheSourceChanges`, `TestRun_VerifyFailureRemovesTarget`, `TestRun_RejectsIncompleteBackend`.
6. PASS — `TestRun_CopiesEverything`, `TestRun_LegacyMigrationMarker`.
7. PASS — attribution and audit assertions in `TestRun_CopiesEverything`; the audit record is read back and its absence fails the run.
8. PASS with deviation — `TestRun_CollectsEveryRowError`, `TestRun_CaseCollidingIDs`; a date property with a time of day is now a row error. Schema-invalid rows are copied but not listed; the report advises `rela-sqlite analyze` (IMPL-MQ0ION).
9. PASS — `TestRun_ListsFilesTheStoreDoesNotRead`.
10. PASS — mode and `.gitignore` assertions; `TestRun_KeepsOwnerOnlyConfig`.

End to end on a copy of `tickets/`: 5492 entities and 6703 relations, equal to
the source file counts; exit 0.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-E8LVR2

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the PR is opened when the user asks for it)
