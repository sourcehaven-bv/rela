---
id: REV-ZE4354
type: review-checklist
title: 'Review: Cancel on a directly-opened create form walks out of the SPA (router.back with no history)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — suite green
- [x] Lint clean
- [x] Coverage maintained

## Code Review

- [x] Run `/code-review` command — fix reviewed and shipped in PR #1326
- [x] All critical review-responses addressed — none raised
- [x] All significant review-responses addressed — none raised
- [x] Self-reviewed the diff for unrelated changes

## Verification

- [x] Acceptance criteria met — Cancel on a directly-opened create form stays inside the SPA
- [x] Manual testing performed — verified against a deep-linked create form with no prior history entry
- [x] No regressions introduced
- [x] Documentation updated — n/a, behavioural fix with no operator-visible configuration

## Retrospective note

Recorded after the fact. This bug merged without a linked review checklist
because the ticket gate did not run on its PR (see BUG-CI7XKP); the gate is
repo-wide, so the gap surfaced on the next PR that did run it. Written now
rather than back-dating a claim that the checklist was followed at the time.
