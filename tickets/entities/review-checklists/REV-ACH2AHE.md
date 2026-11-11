---
id: REV-ACH2AHE
type: review-checklist
title: 'Review: Headless rela-server build for sqlite and postgres in releases'
started: "2026-11-11"
completed: "2026-11-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

Evidence: the full suite with `-race -cover` passes (every package `ok`;
`internal/dataentry` 89.1%). `go-test-coverage` passes (85.8% total). The first
`just coverage-check` run timed out two packages at the default 10m under
machine load (`dataentry` printed PASS first; `sqlitestore` is untouched); rerun
with `-timeout 30m` passed. `just lint`: the maildemo,mailmanual run reports 171
staticcheck SA5011 findings, all in test files this diff does not touch (local
linter built with go1.26.6); the sqlite, postgres, memorybackend and headless
runs on the changed packages report 0 issues. `just comment-lint`: no
unresolvable doc links.

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

**Review Responses:** design review RR-S8QAG7O, RR-B95TD1M; code review
RR-3IDXMWQ (critical), RR-8OP1JO3, RR-47CU37H (significant), RR-0J8FLFU,
RR-QFS1YPT, RR-K1CYFH5, RR-IS9YCVG, RR-JV1OEPE, RR-T2V0HET (minor), RR-ICIKJBP
(nits). All addressed.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: both headless variants build without a frontend build; the `absent`
guard finds none of 128 SPA assets and no editor bundle in either.
2. PASS: live server returns `/` 200 with Link header, `/api/v1/_schema` 200,
UI paths, `/_custom/`, editor assets 404; collab route reports "Co-editing is
not enabled". Pinned by TestHeadless_Router and friends.
3. PASS: untagged suite passes.
4. PASS: `goreleaser check` ok; snapshot release produced all 12 headless
archives (rela-headless-{sqlite,postgres} × 3 OS × 2 arch).
5. PASS: CI `headless` job (build on 3 OSes, vet, tests, refusal of
desktop/docs) and headless lint step; YAML has no duplicate keys.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-12RXQUQ

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: PR not requested yet; opened after done per TKT-UFV01M)

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
