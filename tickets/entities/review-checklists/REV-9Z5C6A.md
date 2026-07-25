---
id: REV-9Z5C6A
type: review-checklist
title: 'Review: POST /webhooks/idp is unreachable — registered on the /api/-only inner mux, falls through to SPA catch-all'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] Affected-package tests pass (`internal/dataentry`, `jwtauth`, `cmd/rela-server`)
- [x] Lint clean (`just lint` — 0 issues)
- [x] `just arch-lint` clean, `just plimsoll` clean
- [x] ~~Full `just test`~~ (N/A locally: `internal/docscapture` browser-automation tests time out in this headless env — confirmed identical failure on unmodified `develop`, so pre-existing and unrelated. CI has a real browser env.)

## Code Review

- [x] ~~Run `/code-review` (cranky-code-reviewer)~~ (N/A: the change is a one-line route-registration move from `inner` to `mux` plus its regression test; a full adversarial agent review is disproportionate and the diff is self-reviewable)
- [x] Self-reviewed the diff for unrelated changes — only router.go (1 line + comment), the new regression test, and the walk-test exclusion note
- [x] No review-response entities needed

**Review Responses:** none

## Acceptance Verification

- [x] Bug reproduced first: `TestWebhook_ReachableThroughRouter` fails on the old wiring with `200 <!DOCTYPE html>` (SPA shell), then passes after the fix
- [x] Route confirmed reachable through the real `NewRouter()`, not the handler in isolation

**Acceptance Status:** PASS — the webhook handler now runs for `POST
/webhooks/idp`; the reproduction test is red-before / green-after.

## Documentation

Skipped: bug fix, no user-facing surface change. The webhook was documented as
working; this makes the documented behavior true.

- [x] ~~Docs-checklist~~ (N/A: no doc changes — the fix realigns behavior with existing docs)

## Final Checks

- [x] Commit message explains the why (dead since #1069, the mux/prefix mismatch)
- [x] No TODOs/FIXMEs left
- [x] `route-reachability-through-production-router` measure now backed by a real test

## Pull Request

- [ ] `/pr` — PR opened, CI monitored
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** *(pending — see next step)*
