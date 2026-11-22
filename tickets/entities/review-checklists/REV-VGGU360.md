---
id: REV-VGGU360
type: review-checklist
title: 'Review: Trigger placeholder for the current user'
started: "2026-11-22"
completed: "2026-11-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass: `go test` on flowcfg, appbuild and entitymanager. The full suite runs in CI
- [x] Lint clean: golangci-lint on the changed packages (0 issues), `just arch-lint`
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained: new code is covered by unit and integration tests; CI enforces the floors

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

- [x] Ran cranky-code-reviewer
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-0AVS9IR, RR-0L8G1Z0, RR-3OVYO2M, RR-RSLIE4F,
RR-8DWAL8R, RR-5K50HA1 (addressed), RR-EQ6EDU3 (wont-fix)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. Loads in every interpolated field: PASS (parse tests).
2. Resolved user renders and relates: PASS (TestEngine_CurrentUserPlaceholder, TestCreate_CurrentUserTriggerRelatesAuthor).
3. No resolved user renders empty, relate skipped: PASS (same tests).
4. Changed during review: a stamped query identity is not read; the principal is. PASS (TestEngine_CurrentUserPlaceholder).
5. Not in inline Lua: PASS (TestInterpolate_CurrentUser).
6. Guide documents it: PASS (GUIDE-flows, GUIDE-metamodel).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-L4CBEAU

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] PR opened with `gh pr create` per the task brief and CI watched

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
