---
id: REV-9UHVGC
type: review-checklist
title: 'Review: Configurable backlinks section with per-type columns'
started: "2026-10-08"
completed: "2026-10-09"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass: `go test -race` on the 28 packages that depend on dataentry, dataentryconfig or apiwire/v1 (all ok); frontend vitest for EntityDetail. The full `just test` was cut short by a full disk, so `/verify` runs the full CI before commit
- [x] Lint clean (`just lint`, arch-lint, plimsoll: 0 issues)
- [x] Comment lint gate clean (`just comment-lint`); `comment-report` shows no finding in the new code
- [x] Coverage maintained: dataentry 85.8% (floor 55), dataentryconfig 92.4% (floor 70)

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
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** Code review: RR-DC5KM3 (significant), RR-68KZPU,
RR-40YL5Z, RR-UQJP2V, RR-M2X8K8, RR-N2R3I3 (minor), RR-YBVNAP, RR-A0UQEN (nit),
all addressed. The 18 design-review RRs are addressed too.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: TestViewMentions_TableGroupsByType (hidden sources absent), demo TKT-002.
2. PASS, with the design-review change: the flag is `incomplete`, not `truncated` (RR-6G9DDN). TestViewMentions_CapAndWhere; SPA note in EntityDetail.mentions.test.ts.
3. PASS: CapAndWhere "where past the first page" and "where matches nothing".
4. PASS: TestViewMentions_TableGroupsByType; frontend per-group headers test.
5. PASS: TestViewMentions_FieldsByType.
6. PASS: TestValidateConfig_MentionsAndByType (21 cases) and the side-panel test.
7. PASS: TestViewMentions_PanelSwitch, the world-absent case, and the frontend panel test.
8. PASS: TestQueryBudget_MentionsViewIsSizeIndependent (7 reads at 10 and 50).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-UKE8Z0

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: a PR is opened only on the user's request)

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
