---
id: REV-Q35QT14
type: review-checklist
title: 'Review: Show each session decision button once'
started: "2026-11-04"
completed: "2026-11-04"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass: frontend `npm run test:run` (387 files, 5849 tests), library vitest with stories (686), e2e `channels.spec.ts` and `kanban-busy.spec.ts` (16). No Go change, so `just test` is not affected
- [x] Lint clean: frontend `npm run lint` (0 errors; the only warning in a changed file is the existing `max-lines` on `OpenChannel.vue`), library `vue-tsc -b` and `npm run check`, e2e lint and typecheck, `just lint-md`. ~~`just lint`~~ (N/A: Go only, no Go change)
- [x] ~~Comment lint gate clean (`just comment-lint`)~~ (N/A: scans `./internal` and `./cmd` only, no Go change)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: Go coverage only, no Go change)

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

- [x] Run `/code-review` command: cranky-code-reviewer and rela-security-reviewer in parallel; the security review found nothing
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes (reverted the repo-wide `rela fmt` rewrites)

**Review Responses:** RR-2N6HFSL, RR-39HAJUL, RR-69EK5AD, RR-9A9U0TO,
RR-B8SHAMZ, RR-GPNEL8V, RR-JGYTUBA, RR-NDF7VB3, RR-NFXMQEM, RR-XKBJKNJ (1
significant, 8 minor, 1 nit; all addressed)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- AC 1 PASS: ChannelsPanel "shows the buttons once, on the deciding message"; e2e `channels.spec.ts` decisions test.
- AC 2 PASS: ChannelsPanel fallback table (cause not held, removed, timer, generation 0); `placeDecisions` unit tests; inbox cause 0 test.
- AC 3 PASS: ChannelsPanel answer presets on the row; `buttonTypesOf` unit tests; InboxWaitingItem presets on the question.
- AC 4 PASS: stories DecisionMode, DecisionModeWithPicker, ButtonTypesSelect, EditButtonTypeKeepsDecision, CancelSelectsDefault; useSessionDecisions refusal and Other-cancel tests.
- AC 5 PASS: stories AllButtonTypes, ReplyFollowsPicker, ReplyWhenAllButtonTypes; useSessionDecisions gap and error tests.
- AC 6 PASS: InboxWaitingItem tests; shared `placeDecisions`.
- AC 7 PASS: Go to decision focus tests (ChannelsPanel, e2e), focus-follow tests incl. two channels, RlSegmentedControl NoneChecked and NoneCheckedArrows, CancelToNoFields.
- AC 8 PASS: RlChannelSession CompactWithGoToDecision, NarrowWithGoToDecision; e2e compact test.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-0CDGCG7

## Final Checks

- [x] Commit message explains the why, not just what (minimal, per project owner)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (next step, after `done`)

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
