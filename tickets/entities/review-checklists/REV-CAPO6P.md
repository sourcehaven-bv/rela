---
id: REV-CAPO6P
type: review-checklist
title: 'Review: Lua rela.attach_render: render, convert and attach'
started: "2026-10-28"
completed: "2026-10-28"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (full suite green under `just coverage-check`; sqlite-tagged sqlitedb and sqlitestore green; postgres-tagged pgstore green against a local scratch database after the rebase onto develop)
- [x] Lint clean (`just lint`) (golangci-lint 0 issues for the default, sqlite, postgres and memorybackend tags; plimsoll and arch-lint clean)
- [x] Comment lint gate clean (`just comment-lint`) (no unresolvable doc links; no advisory findings in the touched files)
- [x] Coverage maintained (`just coverage-check`) (package floors pass)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (cranky-code-reviewer and rela-security-reviewer both ran)
- [x] All critical review-responses addressed (RR-T14RK0)
- [x] All significant review-responses addressed (RR-Z38M50, RR-0GH0RM)
- [x] Self-reviewed the diff for unrelated changes (every file belongs to the ticket; rebased onto develop after #1999 and #2001, migrations renumbered to pg 0036 and sqlite v29)

**Review Responses:** design: RR-0BZDKN, RR-1IQF2X, RR-1V015A, RR-7MGS8W,
RR-97TJV2, RR-9NBAXM, RR-GNT57F, RR-GOKV3S, RR-HUBH5I, RR-IXGRW3, RR-IYDFHJ,
RR-JBEMX6, RR-RR963V, RR-XKMNOL. Code and security: RR-T14RK0, RR-Z38M50,
RR-0GH0RM, RR-BA1DST, RR-A13XZ1, RR-ESFLWV, RR-AVIM3L, RR-M2HDB4, RR-BRWHEY
(addressed); RR-WISFA4 (deferred); RR-M9GDLZ, RR-NO1I2K (wont-fix).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. PASS: an action on an audit entity attaches a PDF of a document (TestAttachRender_ActionAttachesPDFOfDocument; manual run on fs, SQLite and PostgreSQL servers).
2. PASS: the returned sha256, the API `_attachments[].sha256` and the hash of the downloaded bytes agree (same test; storetest hash cases on every backend; manual downloads).
3. PASS: a principal without read access to some rows gets a file without them (TestAttachRender_HiddenRowsAreNotRendered for document and list; OutsideRequestIsGated; manual carol run).
4. PASS: a principal without write access is refused, nothing is stored and the denial is audited (Refusals table; WriteDenialIsAudited; manual dave run).
5. PASS: works on fs, SQLite and PostgreSQL (storetest on all backends; sqlite v29 and pg 0036 backfill tests; manual verification on all three).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-YVBBOU

## Final Checks

- [x] Commit message explains the why, not just what (minimal commit message per user preference, carries the ticket id)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (run after the ticket is done; PR URL and CI status are recorded in GitHub, see TKT-UFV01M)
