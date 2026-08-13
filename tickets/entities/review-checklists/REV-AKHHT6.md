---
id: REV-AKHHT6
type: review-checklist
title: 'Review: CalDAV: declarative caldav: config — symmetrical single-type mapping + fail-fast validation'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** <!-- List IDs of review-response entities created, e.g.,
RR-xxxx -->

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
<!-- For each acceptance criterion, state PASS/FAIL with evidence -->

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: documentation
landed with the feature rather than as a separate tracked task.)
- [x] User-facing documentation updated — `docs/caldav.md` is the deployment
guide (config shape under `caldav.static:`, the field-mapping table,
`priority_map:`, `description: body`, `read_only:`), and
`docs/caldav-clients.md` records per-client compatibility including what each
client does with a refused write.
- [x] ~~Docs-checklist marked as done~~ (N/A: none created.)

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1308 <!-- e.g., https://github.com/org/repo/pull/123 -->

## Evidence (TKT-UGYSC8 — declarative caldav: config)

Reviewed as part of the CalDAV code review covering the whole surface; see
**REV-E7QYNN** for the finding table and the automated-check figures.

No finding landed against `validate_caldav.go`. The reviewer read it as the
counter-example to two of the bugs found elsewhere: `validateCalDAVCompletionReachable`
is cited twice as the check that *prevents* a reverting checkbox, which made it
the reference point for RR-R4SCVX (the same symptom reached through a route no
config change can avoid).

Implementation evidence and the per-AC table are in **IMPL-A1RH8P**.

`just lint` 0 issues, `just arch-lint` OK, package coverage 89.3%.

**Not done:** PR (`/pr`), which is shared across the CalDAV tickets.
