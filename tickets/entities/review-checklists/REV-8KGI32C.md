---
id: REV-8KGI32C
type: review-checklist
title: 'Review: Compact session header by configuration'
started: "2026-11-03"
completed: "2026-11-03"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

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

**Review Responses:** Design review: 16 (RR-V6K0FZV, RR-B0HTXQS, RR-KEN3QD5,
RR-C91FORX, RR-9R4JZTG, RR-0O7MKT7 and ten minor or nit), all addressed. Code
review (cranky-code-reviewer plus the rela security reviewer brief): 16. The one
significant finding (line length) is addressed; the security review found
nothing. Minor M4 (pg schema newer than the binary) is deferred to TKT-D3BQJ5J.
Other wont-fix items carry a reason.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- AC 1 config refused or warned: PASS (`TestChannelsPresentation_Validate`, `TestValidateChannels_SessionDisplay`, `TestChannelsPresentation_SessionScalar`).
- AC 2 Configure may edit it: PASS (allowlist cases).
- AC 3 SPA gets the key: PASS (`TestConfig_ServesChannels`, `channelView.test.ts`, `ChannelsPanel.test.ts`).
- AC 4 previous stored and served: PASS (conformance on memory, file, sqlite and postgres; migration tests; `TestSessionView_Previous`; wire goldens).
- AC 5, 6 compact at 32rem and wider: PASS (stories with play tests; Playwright at 60rem).
- AC 7 below 32rem: PASS (story `CompactNarrow`; Playwright at 24rem).
- AC 8 auto unchanged: PASS (story `AutoWide`; Playwright auto at 60rem and 24rem; the full channels e2e spec passes).
- AC 9 accessibility: PASS (one `aria-current` in the markup, hidden role texts, toggle attributes; stories).
- AC 10 docs: PASS (DOCS-M7SYMI4).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-M7SYMI4

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (next step, after done)

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
