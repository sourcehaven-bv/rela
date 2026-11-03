---
id: IMPL-Y3H858
type: implementation-checklist
title: 'Implementation: Readers could save settings defaults and trigger a git sync'
started: "2026-10-21"
completed: "2026-10-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Unit: `TestMode_Decisions` (the `PermitsProjectWrite` column),
`TestSaveSettings_Gate`, `TestSavePalette_Gate`, `TestThemeLogo_Gate`,
`TestThemeImport_Gate`, `TestHandleGitSync_Gate`, `TestHandleGitStatus_CanSync`
and `TestGitStatusResponse_ConflictFiles`.

Integration: `TestGoldenParity` drives the real router over HTTP with header
identities against the golden fixture's `acl.yaml`. The settings, palette, logo
and theme-import goldens now record 403 for editor, viewer, outsider and
stranger; admin, who holds `settings:edit`, still gets 200.

Edge cases: no `acl.yaml` (allowed), `--read-only` (refused even when the gate
would grant), a broken policy (refused), and a refused call persisting nothing
(no saved defaults or palette, no stored or deleted logo, no git commit).

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Builders: `newTestAppV1`, `newSettingsEditApp`, `gitSyncRepo`,
`logoUploadRequest`, `makeTinyPNG`. Assertions on the permission name use
`acl.PermSettingsEdit` and `acl.PermGitSync`, not string literals.

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: no UI change; `TestGoldenParity` exercises the full HTTP path with header identities, which is what a manual curl run would check)
- [x] ~~Each acceptance criterion verified with test scenario from planning~~ (N/A: bugs have no planning checklist; the fix plan in BUGA-J6R7BG is covered by the tests listed above)
- [x] ~~Edge cases manually verified~~ (N/A: read-only, broken policy and no-policy cases are pinned by the gate tests; no separate manual run was done)

**Verification Evidence:**

Before the fix, a throwaway test on develop with a policy ACL and a read gate
holding no permission got `status=200 commits_added=1` from `handleGitSync`.
After the fix, the same setup gets 403 and no commit (`TestHandleGitSync_Gate`,
case "policy without git:sync refused"). `go test ./internal/acl/...
./internal/access/... ./internal/dataentry/...` passes.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Pattern: `PermitsProjectWrite` sits next to `PermitsGatedUIElement` and
`PermitsElevatedRead` on `access.Mode` and follows the same rule (only a
configured policy consults the holder). The five settings-page writes share one
guard, `appearanceHandler.refuseSettingsEdit`. A 403 names the config-declared
permission, as the root CLAUDE.md asks for a capability. The security review
(rela-security-reviewer) is recorded as review responses.
