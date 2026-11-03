---
id: REV-Y6IOGD
type: review-checklist
title: 'Review: End SSE streams when the verified token expires'
started: "2026-10-20"
completed: "2026-10-20"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): `go test -race` on internal/jwtauth, internal/dataentry and cmd/rela-server; the new tests with `-count=20`; build and vet on the default, `postgres` and `sqlite` tags. CI runs the full suite on the PR
- [x] Lint clean (`just lint`): golangci-lint on the changed packages (0 issues) and `just arch-lint`
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`): new code is covered by the tests above; CI runs the coverage floors on the PR

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent): cranky-code-reviewer and rela-security-reviewer ran on the diff
- [x] All critical review-responses addressed: none raised
- [x] All significant review-responses addressed: none raised
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** 16 review-response entities linked via
`has-review-response`: 8 from the design review and 8 from the code and security
reviews (3 minor, 5 nit). All are addressed; none are open.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist): see PLAN-MUHW0I
- [x] Test evidence documented in implementation checklist: IMPL-KFI5W8

**Acceptance Status:**
- AC1 stream ends at expiry, live connection unregistered: PASS, `TestSSEExpiry_StreamEndsAtExpiry`
- AC2 no expiry, stream stays open: PASS, `TestSSEExpiry_NoExpiryStaysOpen`
- AC3 past expiry, no frames and no registration: PASS, `TestSSEExpiry_PastExpiryWritesNothing`
- AC4 expiry projected and copied: PASS, `TestVerifyAssertion_ProjectsExpiry`, `TestAssertedIdentity_CopiesEveryField`
- AC5 refreshed token keeps the stream owner: PASS, `TestSSEExpiry_RefreshedTokenKeepsStreamOwner`

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated: GUIDE-acl-security, regenerated into `docs/`
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-HK5HYY

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed (none added in the diff)
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (opened after the ticket reaches `done`; see TKT-UFV01M)

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
