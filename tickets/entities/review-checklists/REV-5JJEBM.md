---
id: REV-5JJEBM
type: review-checklist
title: 'Review: Make list rows and kanban cards behave as real links'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

Go: `just test` green across ~110 packages under `-race -shuffle=on`. `just
coverage-check` — package threshold (50%) PASS, total threshold (65%) PASS,
total 78.8% (33517/42547).

`just lint` 0 issues; `just comment-lint` no unresolvable doc links across 11461
comments; `just plimsoll` clean; `just lint-md` 0 issues in 257 files.

Frontend (where this change actually lives): 2066 passed (2066) across 127
files, up from 2052 by the 14 tests added here. `npm run build` — which runs
`vue-tsc -b` — clean. `npm run lint` 0 errors.

The two eslint *warnings* reported in the changed files are pre-existing, not
introduced: both sit on untouched lines (EntityList.vue:85, KanbanView.vue:173)
and were confirmed by stashing both files and re-running lint — the same two
warnings appear against the pristine tree.

**Comment findings.** `just comment-report` lists the advisory rules
(duplication, nil-contract, param-contract, restatement). They are not a merge
gate, but a finding your diff *introduces* should be fixed or suppressed — don't
grow the backlog.

Every rule is a heuristic over prose, so false positives are expected. To
suppress one, prefer the inline form on the declaration line, which travels with
the code and is reviewed in this diff:

```go
func f(p string) {} //commentlint:ignore param-contract  p is contained by Clone
```

Use `.commentlint.yml` (`ignore:` path globs, `allow-phrases:`) only when the
same prose recurs across many sites. A reason is required either way — an
unexplained suppression is a finding nobody can re-evaluate later.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-BWZ0TR (critical), RR-K2PXAT, RR-M8XNQE, RR-P4TDVL,
RR-T7YHWC (significant).

The critical one is worth reading before merge: my own de-duplication refactor
put the anchor's `@click.stop` on the wrapper shared by every column, and Vue
resolves `.stop` at compile time — so the plain columns swallowed clicks and
blocked the row handler. `display: contents` hid it almost perfectly (clicking
cell padding still worked; only the rendered text was dead). Fixed, and pinned
by a test that clicks a non-first cell.

RR-T7YHWC is a *rejected* finding — the nested-anchor case is unreachable
because no dense widget kind emits an anchor. Recorded rather than guarded, so
the reasoning survives.

Self-review found no unrelated changes. It did find two render sites I had
missed — the mobile card layout and the swimlane board — which are now fixed and
independently tested.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| # | criterion | status | evidence |
| --- | --- | --- | --- |
| 1 | list row renders a real anchor | PASS | `renders a real anchor in each row pointing at the entity` |
| 2 | one anchor per row, not per cell | PASS | `carries exactly one anchor per row, not one per cell` |
| 3 | href preserves list state | PASS | `preserves list state in the href so a new tab lands in the same context` |
| 4 | plain click still navigates in place | PASS | `still navigates in place on a plain left click` |
| 5 | kanban card renders a real anchor | PASS | `renders a real anchor per card pointing at the entity` |
| 6 | edit_form boards agree href/click | PASS | `points at the edit form when the board configures one` |
| 7 | drag-and-drop still works | PASS | `leaves the card itself draggable and the anchor not` |
| 8 | mobile card layout links too | PASS | `renders a real anchor on each mobile card title` + `still navigates in place on a plain tap` |
| 9 | swimlane board links too | PASS | `links cards on the swimlane board too` |

Plus three tests from review findings: the non-first-cell click regression and
the two encoding agreements (`%20` not `+`; path segments percent-encoded).

Every one of these was mutation-verified — the code was broken and the test
confirmed to redden, then restored. The per-site mutations (swimlane only,
mobile only) redden only their own test, which is what shows the four render
sites are independently covered rather than one test standing in for all four.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** N/A — no documented behaviour changes. Where a click goes is
unchanged; the affordances this restores (open-in-new-tab, Cmd/middle-click) are
standard link behaviour users already expect, and `docs/data-entry.md` never
documented their absence. Marked in the planning checklist under Documentation
Impact.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI

<!--
Deliberately NOT tracked here: the PR URL and whether CI passed.

Both post-date this checklist. `/pr` requires the ticket to be `done` and
validating clean before it opens the PR, and a `done` review-checklist may have
no unchecked items — so an item asking for the PR URL can only be satisfied by a
PR that does not exist yet. Checking it early would mean asserting "CI passed"
before CI ran, which turns the checklist from evidence into a formality.

GitHub records both authoritatively, and the branch and commit messages carry
the ticket ID, so the ticket-to-PR link is recoverable without duplicating it
here. See TKT-UFV01M. -->
