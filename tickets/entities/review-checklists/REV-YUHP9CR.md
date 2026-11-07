---
id: REV-YUHP9CR
type: review-checklist
title: 'Review: Store IdP-asserted roles per principal for background jobs (R14)'
started: "2026-11-06"
completed: "2026-11-06"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass: archguard, lua, appbuild, scheduler, dataentry, script, idproles, principal, cmd/rela-server and cli pass locally; the full `just test` runs in CI
- [x] Lint clean: golangci-lint is clean on every changed package; `just arch-lint` and `just plimsoll` pass
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained: internal/idproles is about 95% against a floor of 90; `just coverage-check` runs in CI

## Code Review

- [x] Run `/code-review` command: rela-security-reviewer and cranky-code-reviewer ran on the diff
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed: RR-Q6JOPA3, RR-K6PT93R, RR-NE4FZCL
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-Q6JOPA3, RR-K6PT93R, RR-NE4FZCL, RR-HRSDTWH
(addressed); RR-PX6L3P5, RR-1A4C66C (minor, deferred with reasons)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- Webhook stores asserted roles for the verified subject only: PASS (TestWebhook_StoresAssertedRoles)
- for_each children hold the mapped roles without a JWT: PASS (TestScheduledForEachPrincipal_StoredRoles)
- `rela.principals_with_role` returns readable holders only: PASS (TestPrincipalsWithRole, TestRoleHolders_PrincipalsWithRole)

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-9C4PHC2

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
