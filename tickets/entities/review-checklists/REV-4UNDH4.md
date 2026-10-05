---
id: REV-4UNDH4
type: review-checklist
title: 'Review: Tracer and analyze skip faced types'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test ./internal/...`, plus tagged vets for postgres, sqlite and memorybackend)
- [x] Lint clean (golangci-lint 0 issues; arch-lint and plimsoll clean)
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

- [x] Run `/code-review` command (cranky-code-reviewer and rela-security-reviewer)
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (RR-M022EY deferred to TKT-5LW875 with reason)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-XL9963, RR-M022EY, RR-F682HK, RR-6GBOMS, RR-C63Q6U,
RR-1WG8TH, RR-JPDD6K, RR-GLEBGD, RR-OB91P8, RR-3PA943, RR-BEYMCU, RR-74732S,
RR-5C37DB, RR-17DICN, RR-KQR8OM, RR-RDOP8H, RR-03KDKL

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- Faced types appear in orphans, duplicates, unique, gaps, cardinality, trace
  and path: PASS (analysis/faces_analyze_test.go, tracer/faces_test.go).
- Each report states its coverage: PASS (cli/analyze_face_test.go,
  mcp/analyze_face_test.go).
- A4, a principal who cannot read one face, on CLI, MCP and data-entry:
  PASS (visibility/facegate_test.go, dataentry/analyze_face_test.go,
  mcp/analyze_face_test.go).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug)
- [x] User-facing documentation updated (docs/cli-reference.md, rela analyze coverage)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug)

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the PR targets faces-intrinsic and is opened with gh after this checklist; see TKT-UFV01M)

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
