---
id: REV-DOCASSERT
type: review-checklist
title: 'Review: Executable manuals — assertions in the rela-docs doc language'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

`go test ./internal/...` clean; `just lint` 0 issues; commentlint "no
unresolvable doc links across 11130 comments"; coverage PASS on both the
package floor and the total. Also `just arch-lint` OK, `just plimsoll` rc=0,
and `go build -tags postgres ./...` clean (the api{} client has a build-tagged
postgres variant that must fail loud rather than seed a live database).

`just comment-report` introduces no new advisory findings on the new files.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-DOCA01 through RR-DOCA10 — 3 critical, 4 significant,
3 minor, all `addressed`.

The three critical findings were one class: an identifier that silently
resolved to a benign default, so a typo made the assertion pass forever
(`who=` unassigned is refused by construction; `type=` unknown is an empty
set; `as=` unknown fell back to a privileged principal). Each is now resolved
against the thing that defines it.

One unrelated change is deliberate and called out in its commit: the
`docscapture` test skip guard stat'd `metamodel.yaml`, renamed to
`schema.yaml` in TKT-FNARO6, so those tests had been skipping silently.
Fixing it was necessary to test this feature at all, and raised package
coverage 27% -> 46%.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- "A call that asserts nothing is an ERROR" — PASS. Enforced on every verb,
  including a misspelled key beside a valid claim (which the rule alone does
  not catch), and pinned by compiling mutants.
- "Prefer `exactly` for lists" — PASS. `exactly` is implemented, documented as
  the recommended form with the relation-leak bug as the rationale, and
  `exactly={}` is a real claim distinct from no claim.
- "A refusal assertion needs a positive control" — PASS. `permits{}` is the
  paired form; the negative-control table asserts that wrong claims turn the
  build red; and the end-to-end proof widened `viewer` in `acl.yaml` and
  confirmed the handbook stops building, then restored it and confirmed green.
- "Failure output prints the page and the seed" — PARTIAL. Every failure
  prints the actual observed state (the seeded id set, the response body and
  status, the rule that fired). The literal "page" half belongs to the browser
  verbs, which are not in this slice.
- Scope items 1, 2 (browser claims), 5 (`world=`) and 6 (multi-backend) are
  NOT delivered; each is recorded in the ticket with the reason. Item 5 is
  blocked on the worlds epic (content states are not on develop); item 6 is
  not reachable while seeding is unavailable on the postgres build.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-DOCASSERT

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
