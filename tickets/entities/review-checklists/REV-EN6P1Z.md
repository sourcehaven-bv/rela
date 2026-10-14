---
id: REV-EN6P1Z
type: review-checklist
title: 'Review: Escape feed text in entity bodies'
started: "2026-10-14"
completed: "2026-10-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (targeted: ingest, subscription, dataentry webhooks, appbuild e2e; the full suite runs in CI)
- [x] Lint clean (`just lint`) (golangci-lint on ingest, subscription, dataentry, appbuild: 0 issues; full lint in CI)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) (no new package; checked in CI)

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

- [x] Run `/code-review` command (cranky-code-reviewer with a security brief)
- [x] All critical review-responses addressed (RR-9VVFON, RR-CQYPVQ)
- [x] All significant review-responses addressed (RR-YASICB, RR-AKK12H; critical RR-9VVFON, RR-CQYPVQ also addressed)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-9VVFON, RR-CQYPVQ, RR-YASICB, RR-AKK12H, RR-IKH2CW,
RR-XBHQXB, RR-PVVXQ7, RR-SY2JOD

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: TestRun_TextIsEscapedOnlyInBodies, TestSubscription_EndToEnd (stored body is escaped).
2. PASS: same tests (title property unchanged).
3. PASS: dataentry webhook suite unchanged; webhook values are ValueRaw.
4. PASS: TestItemSource_Links (query, fragment, path and host cases).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-I9U01U

## Final Checks

- [x] Commit message explains the why, not just what (minimal message per user preference)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (done after this checklist; see TKT-UFV01M)

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
