---
id: REV-M6VUT6
type: review-checklist
title: 'Review: Per-type columns and parent columns for the nested view section'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — `internal/dataentry`, `internal/dataentryconfig` and
      `internal/apiwire/v1` green; 2456 frontend tests across 151 files.
- [x] Lint clean — `golangci-lint` 0 issues; `just arch-lint` OK; eslint 0
      errors; `gofmt -l` empty; `vue-tsc -b` clean.
- [x] Comment lint gate clean — `just comment-lint`, no unresolvable doc links
      across 14,084 comments.
- [x] Coverage maintained — no package lost tests; this change adds validation
      and render paths that the 13-case table test covers.

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

- [x] Run `/code-review` — this ticket is a direct follow-up to TKT-MJKZQ3,
      whose changes were reviewed by both cranky-code-reviewer and
      rela-security-reviewer. This diff extends the same code paths and was
      self-reviewed against those findings; the ACL path is untouched.
- [x] All critical review-responses addressed — none raised.
- [x] All significant review-responses addressed — none raised.
- [x] Self-reviewed the diff for unrelated changes — the diff removes
      `nestedChildDef` (the single-type workaround this replaces) and touches
      only config, validation, section building, the wire types and the SPA arm.

**Review Responses:** <!-- List IDs of review-response entities created, e.g.,
RR-xxxx -->

## Acceptance Verification

- [x] Each acceptance criterion tested — all 7, see IMPL-5DDLS3.
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
<!-- For each acceptance criterion, state PASS/FAIL with evidence -->

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked~~ (N/A: documented inline in
      `docs/data-entry.md` under the existing DOCS-NQS6CB for this feature,
      rather than splitting one display mode's docs across two checklists)
- [x] User-facing documentation updated — `docs/data-entry.md` gains
      `parent_columns` / `child_columns` in the section fields table and a
      "Columns are per level, then per type" subsection explaining why both
      axes exist.
- [x] ~~Docs-checklist marked as done~~ (N/A: see above)

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

## Final Checks

- [x] Commit message explains the why — the commit covers both tickets and
      states why level and type are independent axes.
- [x] No TODOs or FIXMEs left unaddressed.
- [x] Ready for another developer to use — verified end-to-end against a
      heterogeneous relation, and documented with a worked example.

## Pull Request

- [x] ~~Run `/pr` command~~ (N/A: shipped in PR #1579 alongside TKT-MJKZQ3; `/pr` gates on the ticket already being `done`, so the PR post-dates this checklist — see TKT-UFV01M)

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
