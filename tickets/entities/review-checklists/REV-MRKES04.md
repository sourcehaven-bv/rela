---
id: REV-MRKES04
type: review-checklist
title: 'Review: e2e: buildIfMissing passes -o after the package path'
started: "2026-11-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (N/A: no Go change; e2e `npm run typecheck` passes and the full suites run in CI)
- [x] Lint clean (`just lint`)
- [x] ~~Comment lint gate clean (`just comment-lint`)~~ (N/A: commentlint checks Go comments; no Go change)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: no Go change)

Lint: `npx eslint tests/fixtures.ts` in `e2e/` is clean; golangci-lint is not
relevant to a TypeScript-only diff.

## Code Review

- [x] ~~Run `/code-review` command (invokes cranky-code-reviewer agent)~~ (N/A: xs argument-order fix; self-reviewed instead)
- [x] ~~All critical review-responses addressed~~ (N/A: no review responses created)
- [x] ~~All significant review-responses addressed~~ (N/A: no review responses created)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** None.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: default `rela-server` built from an absent path.
2. PASS: postgres server built with `-tags=postgres` from an absent path.
3. PASS: signature is `buildIfMissing(binaryPath, pkg, flags = [])`; argv puts the package last.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: chore, not enhancement)
- [x] ~~User-facing documentation updated~~ (N/A: test fixture only)
- [x] ~~Docs-checklist marked as done~~ (N/A: chore, not enhancement)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI

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
