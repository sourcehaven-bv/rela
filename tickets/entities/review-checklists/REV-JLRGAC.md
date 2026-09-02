---
id: REV-JLRGAC
type: review-checklist
title: 'Review: Mail template compatibility hardening + a mailrender-backed render path for Lua'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

`go test ./...` green · `golangci-lint` 0 issues · `arch-lint` "OK - No warnings
found" · `comment-lint` no unresolvable doc links across 11,675 comments ·
coverage 78.6% total, all floors satisfied. Affected packages: mailrender 90.9%
(floor 85), lua 86.5% (floor 80), mail 86.7% (floor 85), mailtemplate 85.5%.

**One coverage note worth recording.** A background `coverage-check` reported
success in its notification while its output had actually failed, on a corrupted
`coverage.out` — one malformed line out of 33,944, two module paths spliced
mid-token, from `go test -race -shuffle=on` writing the profile concurrently.
Zero corrupted lines touched this ticket's packages; the implicated files were
`internal/jobs/retry.go` and `internal/dataentry/actions.go`. Re-running cleanly
passed. **Not attributable to this branch**, but two pre-existing issues were
surfaced and are worth separate tickets: the `just` recipe reported exit 1 while
the wrapper recorded exit 0 (a genuine coverage failure could be swallowed), and
`test-coverage` is flaky under `-shuffle=on`.

**Comment findings.** `just comment-report` flagged one `duplication` finding
that this diff introduced: `BaseURLCarrier`'s doc restated
`RecipientPolicyCarrier`'s rationale. **Fixed, not suppressed** — it now cites
the other and records the one place the two genuinely differ (an absent
recipient policy must deny; an absent base URL is safely unknown). No
suppressions were added anywhere in this diff.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

Two reviewers ran in parallel: **cranky-code-reviewer** (quality) and
**rela-security-reviewer** (security). Both verified claims by execution rather
than reading — the quality reviewer mutation-tested the template seven ways and
confirmed every regression was caught by the test that claims to own it.

**Review Responses:** RR-L9GJBS, RR-USNDMB, RR-9CQTQO, RR-CJKMAG

| ID | Severity | Status |
|---|---|---|
| RR-L9GJBS | minor | addressed — security review, no findings |
| RR-USNDMB | significant | addressed — nil hole in `rows` emitted `<tr></tr>` |
| RR-9CQTQO | significant | addressed — `.prose` guard too thin; footer margin |
| RR-CJKMAG | minor | addressed — ragged rows, palette keys, sweep gap, staleness |

**No critical findings.** Both significant findings are fixed with tests; both
were real defects producing silently wrong output rather than style objections.

Both reviewers independently confirmed the two load-bearing architecture claims:
`go list -deps ./internal/mailrender` returns only itself (a true leaf), and
`internal/lua` still has no dependency on `internal/mail` — so `lua →
mailrender` cannot close a cycle.

**Self-review for unrelated changes:** one intentional out-of-scope fix, both
recorded rather than slipped in. `.pad p` never reached the footer (pre-existing
on `develop`); the `.prose` refactor was the natural moment, since one rule now
covers all three markdown sites where `.pad` covered two. And a stale line in
`docs/mail.md` described `mail.send` in the future tense though it shipped in
TKT-DS1CR6. Nothing else in the diff is outside the ticket.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** all 15 PASS. Per-criterion evidence is in IMPL-FJC421.

The reviewer independently mutation-tested seven of them rather than trusting
the test names: padded div, restored `role="presentation"`, dropped
`scope="col"`, removed gap row with margin restored, added `color-scheme` meta,
reintroduced the `.wrap td` bug, and dropped logo dimensions. Each was caught by
the test that claims to own it. `TestCompat_GuardActuallyFires` — testing the
detector itself against a synthetic violation — was called out as the step most
people skip.

AC-8 carries a deliberate change worth naming: `TestRender_KeepsInlineStyles`
previously asserted `<style>` was gone entirely, which is no longer correct now
that an `@media` block must survive. Rather than weaken it, it strips the
at-rule and asserts nothing survives *outside* it — so it still fails if
inlining stops running. Weakening it to "contains `style=`" would have retired
the canary.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-YA1SXD

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

No TODOs or FIXMEs introduced. Working tree contains only the intended changes —
both reviewers restored their probe files and confirmed the tree byte-identical
to how they found it.

**Known limits, stated rather than papered over:**

- The dark-mode `@media` block's *presence* is testable; whether it *looks
right* in each targeted client is not. Verified visually in both schemes in a
browser, which is a proxy for a real mail client, not a substitute. The dataset
is a compatibility **floor**.
- The vendored dataset's `prefers-color-scheme` row was last tested 2023-03-08.
The client tiers are broadly stable but exact per-client claims are approximate.
- `data-ogsc`/`data-ogsb` Outlook.com targeting is deliberately not done, and
recorded as a decision rather than an oversight.

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
