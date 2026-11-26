---
id: REV-FNMMPIG
type: review-checklist
title: 'Review: Options to hide type and ID on cards and mention lists'
started: "2026-11-26"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (ran the changed packages: dataentryconfig, dataentry, apiwire, migration, and vitest on components/entity; the full suite runs in CI)
- [x] Lint clean (`just lint`) (golangci-lint on the changed packages, plimsoll, arch-lint, vue-tsc, and archplan against develop: 0 worse)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) (new code has unit tests; the floor check runs in CI)

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
- [x] Self-reviewed the diff for unrelated changes (the migration fix is needed for entity_views keys to load)

**Review Responses:** RR-1H5O52N (critical, addressed), RR-0LW0N03 and RR-165JH2O (significant, addressed), RR-11XYG1K (minor, addressed), RR-FOP0CDR (nit, addressed), RR-30CIHZA (nit, wont-fix). Design review: RR-Q7B99XW (addressed), RR-BGC2ROA (deferred).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- hide_type and hide_id on list, cards and content sections: PASS (views_mentions_test.go, EntityDetail.mentions.test.ts, browser check on ticket view).
- hide_id on nested sections: PASS (EntityDetail tests).
- A row with a blank title keeps its ID: PASS (T-2 and T-3 fixtures).
- mentioned_by_hide_id on the Mentioned-by panel, face suffix kept: PASS (MentionedByPanel.test.ts; browser check on label/bug shows the title without TKT-001).
- Invalid placements refused at load: PASS (validate_hideidentity_test.go).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-F7CX71P

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the PR is already open and Auto-fix watches its CI)

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
