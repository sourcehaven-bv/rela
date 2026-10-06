---
id: REV-4RSHA5
type: review-checklist
title: 'Review: rela db dump / rela db load: export and import a project''s config (and data) to and from rela.db'
started: "2026-10-04"
completed: "2026-10-04"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

All four passed on 2026-10-04. The sqlite-tagged tests named in PLAN-I8RHS9 were
run separately with `go test -tags sqlite` and pass.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** this ticket's commits were reviewed as part of the
branch-wide develop...HEAD review (cranky-code-reviewer and
rela-security-reviewer) recorded on TKT-FGIWPE. Addressed: RR-EHD12P (critical),
RR-UQWLZD, RR-IZBQWW, RR-Z9FDWK, RR-HV99QA (significant), RR-UPQW2E, RR-BRTZYB.
Deferred with reason: RR-HRWXC6, RR-44YDTU, RR-BZ57ZI. Won't fix: RR-BNQKEW.
None of the findings concerns `rela db load` or `rela db dump`; no new review
was run for this ticket.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** criteria 1 and 2 of PLAN-I8RHS9 PASS; evidence in
IMPL-BE2TQ5 and the named tests. Config `load` writes no audit record, unlike
the description; only the `--data` import is audited (see PLAN-I8RHS9).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-GQ2O9E

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A here: the branch ships as one PR for FEAT-UP14BT, opened after all its tickets are done)
