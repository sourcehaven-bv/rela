---
id: REV-1DNV3R
type: review-checklist
title: 'Review: Owned items: relation targets that live only on the parent page'
started: "2026-10-06"
status: done
completed: "2026-10-06"
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

**Review Responses:** cranky-code-reviewer found 3 significant, 8 minor and 4
nits; rela-security-reviewer found 3 minor and nothing critical or
significant. All 17 are recorded as review-responses linked to TKT-QO14GB and
addressed (two security findings are documented residuals in
`docs/acl-security.md`).

Gates after the fixes: `go test ./...` passes; `go test -tags sqlite` for
entitymanager and the stores passes; pgstore conformance and the owning tests
passed against a local database; `golangci-lint run ./...`, `just plimsoll`,
`just arch-lint` and `just comment-lint` are clean; `just coverage-check`
passes (82.7%); frontend typecheck passes, lint has 0 errors, and all 3700
vitest tests pass.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- AC1 PASS: `internal/metamodel/owning_test.go`.
- AC2, AC3 PASS: `TestCreateRelation_OwningRules`,
  `TestAutomationCreateRelation_OwningRules`,
  `TestOwner_FallbackWriteAppliesOwningRules`, `TestCopy_OwningEdgeFollowsTheRules`.
- AC4 PASS: `TestDeleteEntity_TakesOwnedEntities`, `_OwnedWithoutCascade`,
  `TestDeleteEntity_PartialCascadeLabelsOwnedRelations`.
- AC5 PASS: `TestDeleteEntity_OwnedDenialDeletesNothing`,
  `TestSoftDelete_OwnedDenialMarksNothing`.
- AC6 PASS: `TestSoftDelete_OwnedEntitiesRoundTrip`,
  `TestRestore_OwnedGrantThroughOwner`,
  `TestRestore_RefusesOwningEdgeTheLiveGraphForbids`,
  `TestSoftDelete_PartialFailureIsAudited`.
- AC7, AC9 PASS: `TestOwner_ListAndSearchRows`,
  `TestOwner_ViewCarriesOwnerAndRelatedSection`, `TestOwner_ReadBudget`.
- AC8 PASS: `EntityDetail.owner.test.ts` and `e2e/tests/owned-items.spec.ts`.
- AC10 PASS: an entity with no owning edge carries no `_owner`
  (`TestOwner_ListAndSearchRows`), so removing the edge makes it a normal
  entity.
- AC11 PASS: `TestOwner_Irregular`, `internal/analysis/owning_test.go`.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-Y0EYT4

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A here: `/pr` runs after the ticket is done; see the note below)

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
