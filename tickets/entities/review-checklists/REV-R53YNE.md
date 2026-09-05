---
id: REV-R53YNE
type: review-checklist
title: 'Review: role_relations verb-key fail-open'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just check`)
- [x] Lint clean (included in `just check`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

`just check` exit 0. `just arch-lint` → no warnings. `just comment-lint` → no
unresolvable doc links across 11650 comments. `just coverage-check` → package
floor (50%) and total (65%) both satisfied; total 78.6%. `just docs-check`
passes on the committed tree (it diffs the working tree, so it must be run
after committing the regenerated `docs/`).

## Code Review

- [x] ~~Run `/code-review` command (invokes cranky-code-reviewer agent)~~
(N/A: not run. This change is one strict unmarshaller mirroring an existing
one in the same file plus documentation prose; it adds no evaluation path and
alters no decision logic. Recorded here rather than silently skipped.)
- [x] ~~All critical review-responses addressed~~ (N/A: no review run, none
raised)
- [x] ~~All significant review-responses addressed~~ (N/A: no review run, none
raised)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** none

Self-review: the diff touches `internal/acl/policy.go` (one struct's godoc plus
a new `UnmarshalYAML`), `internal/acl/policy_test.go` (two new tests), the
`GUIDE-acl-security` source entity, and the regenerated `docs/acl-security.md`.
No unrelated changes.

The security model is deliberately untouched. No evaluation path, ceiling, or
`aclaudit` check changed, so `RequiresPermission != ""` remains the boolean
"is this gated" that A2/A6/A7, `EffectiveMembershipRelation` and the boot
warning all key on — which is exactly why the fix refuses the per-verb form
rather than implementing it.

## Acceptance Verification

- [x] Each acceptance criterion tested
- [x] Test evidence documented below

**Acceptance Status:**

1. *A write-verb key under `role_relations:` is refused at load* — **PASS**.
`TestLoadPolicy_RoleRelationsVerbKey_Rejected` covers `create`/`update`/
`delete`/`read` (each asserted to mention `relation_grants`) plus a typo'd
`confer:` asserting "unknown key". Verified end-to-end against the built
binary on a scratch project: `rela acl audit` and `rela list` both refuse,
proving the failure is at `appbuild` policy load and therefore blocks every
entry point, not just the linter.

2. *The supported keys still load* — **PASS**.
`TestLoadPolicy_RoleRelationsSupportedKeys` round-trips `confers` and
`requires_permission`, so the strict unmarshaller did not narrow the real
surface.

3. *No existing config breaks* — **PASS**. No `acl.yaml` in the repo uses
`role_relations` at all (`prototypes/data-entry/project/acl.yaml` is the only
policy file). The new error can only fire on config that was already
silently broken, since a dropped verb key never had an effect.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix, not
an enhancement)
- [x] User-facing documentation updated — this bug was *half* a documentation
defect, so the docs are part of the fix rather than a follow-up: the
`GUIDE-acl-security` guide gained a "The gate covers every verb" section
explaining why the gate is flat, and its example permission was renamed
`member-of:create` → `delegate-membership` because the old name read exactly
like a per-verb key and was the most likely source of the broken config.
`docs/acl-security.md` regenerated with `just docs`.
- [x] ~~Docs-checklist marked as done~~ (N/A: no docs-checklist needed)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
