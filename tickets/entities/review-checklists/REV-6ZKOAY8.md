---
id: REV-6ZKOAY8
type: review-checklist
title: 'Review: idp-sync.lua keeps roles of a deactivated user and calls a missing Pratique route'
started: "2026-11-07"
completed: "2026-11-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test ./internal/lua/` and the dataentry webhook tests; the change touches only an example script and its tests, full suite runs in CI)
- [x] Lint clean (`golangci-lint run ./internal/lua/...`, the only changed Go package)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained~~ (N/A: no non-test Go code changed)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (RR-0RAAIAM 404 clears roles, RR-PYMYTKX org not pinned)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-0RAAIAM, RR-PYMYTKX (significant); RR-0JGLHMG,
RR-TQBEFLO, RR-XATO1LY, RR-9PZFZCP, RR-QMP5824 (minor); RR-4VLZYEE, RR-C7D2N89
(nit). All addressed.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- Roles cleared for a non-active user: PASS (`TestIdpSyncAction_ClearsRolesUnlessActive`, `TestIdpSyncAction_DeactivatesProvisionedMember`)
- Action uses an existing Pratique endpoint (member list): PASS (`TestIdpSyncAction` against a stub serving only Pratique's routes)

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix; docs/idp-webhook-provisioning.md updated in the same change)
- [x] ~~User-facing documentation updated~~ (N/A: bug, docs updated anyway)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
