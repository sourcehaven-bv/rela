---
id: REV-K11JJE
type: review-checklist
title: 'Review: Bounded relation replace skipped the audit log on a type mismatch'
started: "2026-10-22"
completed: "2026-10-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (touched packages with `-race`: dataentry, entitymanager, archguard; full suite runs in CI)
- [x] Lint clean (`just lint`) (golangci-lint, all three tag sets, on touched packages)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) (net test lines added, dead code removed; CI enforces floors)

**Comment findings.** `just comment-report` lists the advisory rules
(duplication, nil-contract, param-contract, restatement). They are not a merge
gate, but a finding your diff *introduces* should be fixed or suppressed — don't
grow the backlog.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-WRJGO5, RR-GVGN6I, RR-M40KU6, RR-KV3FD5, RR-GUCLTN,
RR-6GPJXK (wont-fix nit), RR-33L6YY. No critical or significant findings. The
rela-security-reviewer found no issues.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- PASS: a type-mismatched create on a bounded relation is audited with the
principal (`TestPatchRelations_DisallowedTypeOnBoundedRelationAudited`, fails on
develop).
- PASS: no direct store write remains in `internal/dataentry` write paths.
- PASS: bound, ACL and owning rules still apply
(`TestPatchRelations_DisallowedTypeOverBoundIs422`,
`TestPatchRelations_DisallowedTypeWithDeniedRemoveWritesNothing`,
`TestReplaceRelations_TolerateTypeMismatch`).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix, no user-facing change)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
