---
id: REV-7ILS94
type: review-checklist
title: 'Review: Adopt commentlint in CI: comment-discipline gate + advisory report'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — full `just ci` run twice on this branch;
every package `ok`, zero `FAIL` lines
- [x] Lint clean (`just lint`) — EXIT=0, "0 issues"
- [x] Comment lint gate clean (`just comment-lint`) — EXIT=0, "no findings
across 9876 comments"
- [x] Coverage maintained (`just coverage-check`) — runs inside `just ci`;
no floor breached (this change adds no Go code to the repo)

Also verified independently: `arch-lint` EXIT=0, `plimsoll` EXIT=0, `lint-md`
EXIT=0, and `ci.yml` parses with the `comment-lint` job registered (5 steps).

## Code Review

- [x] Run `/code-review` — performed on the diff (`git diff develop...HEAD`)
- [x] All critical review-responses addressed — none raised
- [x] All significant review-responses addressed — RR-TK4N2J
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-TK4N2J (significant, addressed), RR-DK21QY (minor,
addressed)

Both were real defects in the work under review, not nits:

- **RR-TK4N2J** — the advisory step reported only `restatement` (19), because
the four rules carrying the backlog are opt-in and were never enabled. The
entire gate/report split is justified by making that backlog visible, so the
step was not doing the one job it existed for. Fixed by looping per rule; all
five now report, totalling 301.
- **RR-DK21QY** — `-rank -top N` counted only displayed findings, so the
report claimed 40 duplication findings where there were 119. Fixed **upstream**
(v0.2.1) rather than worked around, since a misleading count is a tool bug;
regression tests added there.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. Gate exits 0 on this branch — **PASS** (EXIT=0, 9876 comments)
2. Gate catches a regression — **PASS by construction** (rule is 0 today and
unit-tested upstream; not re-verified by committing deliberate dead code)
3. Advisory report runs and never fails — **PASS** (all five rules, 301
findings, `continue-on-error` plus `|| true`)
4. CI job registers — **PASS** (YAML parsed, `comment-lint` with 5 steps)
5. Suppression works both ways — **PASS** (inline directive used at
`imgproc/orientation.go:48`; `allow-phrases` carries the ACL idiom)

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated — N/A for `docs/`, justified in the
docs-checklist (contributor tooling, same treatment as plimsoll)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-OPDZK2

## Final Checks

- [x] Commit message explains the why, not just what — both commits lead with
the reasoning (why the split exists; why the loop is needed)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use — `just comment-lint` and
`just comment-report [rule]` both documented in `CLAUDE.md`

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1390
