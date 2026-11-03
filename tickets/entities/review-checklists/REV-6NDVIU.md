---
id: REV-6NDVIU
type: review-checklist
title: 'Review: Live views 1/3: sequence-numbered change feed with SSE resume'
started: "2026-10-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (changed packages: `go test -race ./internal/dataentry/ ./internal/changefeed/` pass; frontend `useEvents.test.ts` and typecheck pass)
- [x] Lint clean (`just lint`) (golangci-lint on changed packages with default, postgres, sqlite and maildemo tags: 0 issues; arch-lint, plimsoll, markdownlint clean)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) (new package changefeed 95.1%, floor set to 90; dataentry gained tests for all new code)

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (plus rela-security-reviewer)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (RR-TZ8UK5)
- [x] Self-reviewed the diff for unrelated changes (Realtime Storybook mockups in rela-components are separate work from this session and belong in their own commit)

**Review Responses:** RR-TZ8UK5 (significant), RR-K072YL, RR-5772E8,
RR-REQI95 (minor), RR-EGTWJ3 (nit); all addressed.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist (IMPL-E9N5S3)

**Acceptance Status:**

- AC1 ready frame carries a token: PASS (TestSSEResume_ReadyFrameCarriesToken, manual curl)
- AC2 missed types replayed via header or query: PASS (TestSSEResume_ReplaysMissedTypes, manual curl)
- AC3 ready token does not skip replay: PASS (TestSSEResume_ReadyTokenDoesNotSkipReplay)
- AC4 unusable token resyncs: PASS (TestSSEResume_UnusableTokenResyncs, manual curl)
- AC5 hidden types withheld live and on resume: PASS (TestSSEResume_HiddenTypesWithheld)
- AC6 Retire ends with config-changed: PASS (TestSSEResume_RetireEndsWithConfigChanged, TestSSEResume_ConfigChangedEndsStream)
- SPA reconnects with the last token: PASS (useEvents.test.ts resume token cases)

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated (api-reference.md, acl-security.md)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-3TE737

## Final Checks

- [x] ~~Commit message explains the why, not just what~~ (N/A: not committed yet; the user commits on request)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: PR waits for the user to ask; it follows done by design)

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
