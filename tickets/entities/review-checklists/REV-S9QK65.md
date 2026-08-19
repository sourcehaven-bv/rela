---
id: REV-S9QK65
type: review-checklist
title: 'Review: acl audit A7 reports rela''s own built-in permissions (history:read) as dead, and suggests a fix that breaks a working grant'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A locally: the recipe
cannot complete on this machine — `internal/dataentry` needs 706s under `-race
-shuffle=on` against Go's 10m per-package default, so the run dies before
reaching the threshold stage. Verified per-package instead: `internal/acl`
80.1%, `internal/aclaudit` 91.2%, `internal/cli` 38.7% against a floor of 30.
Full gate deferred to CI.)

`just lint` → 0 issues. `just arch-lint` → no warnings (confirms aclaudit still
imports only `internal/acl`). Full suite green under `-race -shuffle=on`.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-7V4NT9 (significant, addressed)

The reviewer found no critical issues. It raised one significant finding against
this bug, and it was a real one that invalidated a claim I had made in the
implementation checklist.

**RR-7V4NT9** — `BuiltinPermissions()` registration was unenforced. I had
written that registration means "a newly added global constant fails this test."
It did not: `TestAudit_A7_BuiltinPermissionsAreNotDead` iterates the registry,
so an omitted constant is invisible to it. I verified the reviewer's claim
independently by adding an unregistered `PermMutationProbe` constant — both
packages stayed green, reproducing BUG-NRCJ9E exactly. Fixed with
`permguard_test.go`, a source-scanning guard modelled on the existing
`ceilingguard_test.go` precedent in the same package.

Self-review: the diff touches only `internal/acl`, `internal/aclaudit`,
`internal/cli` plus tracker entities. No unrelated changes.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. *Reproduction emits no A7 finding* — **PASS**. End-to-end against the built
binary: `✓ ACL audit: no findings.` Reverting the fix restores the exact
reported message.
2. *A genuinely dead permission is still reported* — **PASS**.
`TestAudit_A7_BuiltinDoesNotMaskRealDeadPermission`, plus an end-to-end run
where `report:sales` is reported while `history:read` is not.
3. *A newly added global constant fails loudly if unregistered* — **PASS**
(only after RR-7V4NT9). Mutation-verified: the unregistered-constant case that
was green before now fails naming file, line, constant and both remediation
options.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix, not an
enhancement)
- [x] ~~User-facing documentation updated~~ (N/A: `docs/acl-security.md:78`
mentions A7 only as "dead permissions" in a summary list of finding categories;
it never describes the check's scope, so it stayed accurate. Searched `docs/`
for any detailed A7 treatment — none exists.)
- [x] ~~Docs-checklist marked as done~~ (N/A: no docs-checklist needed)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1375
