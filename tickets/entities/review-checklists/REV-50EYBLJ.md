---
id: REV-50EYBLJ
type: review-checklist
title: 'Review: Suspended, archived and ended-session states'
started: "2026-11-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (full vitest, vue-tsc, go test for the changed packages; the full suite runs in CI)
- [x] Lint clean (eslint, golangci-lint, go-arch-lint on the changed packages)
- [x] Comment lint gate clean
- [x] ~~Coverage maintained~~ (N/A locally: new code has unit tests; CI checks the floors)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (RR-2S4GYZT)
- [x] All significant review-responses addressed (RR-C1J6NRX, RR-9PSKZ22, RR-DNDM5JV, RR-OHC0B95, RR-KXMXLW1, RR-S75WCAW, RR-MDOCPSJ, RR-8452XIH)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-2S4GYZT, RR-C1J6NRX, RR-9PSKZ22, RR-DNDM5JV, RR-MN916OA, RR-OHC0B95, RR-KXMXLW1, RR-S75WCAW, RR-MDOCPSJ, RR-8452XIH, RR-MLK84NT, RR-50ZLHKC, RR-A5QSJW4, RR-2QCMJH2, RR-AG6DBZ6, RR-RKB3942, RR-CRBYQPU, RR-T7DLKR5

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- Codes per tenant state: PASS (TestHost_FailsClosed, TestProvision_StatusRoute)
- Full pages per refusal: PASS (WorkspaceAccessScreen.test.ts)
- Sign in again with login_url: PASS (workspaceAccess.test.ts, WorkspaceAccessScreen.test.ts)
- No identity: PASS (workspaceAccess.test.ts, WorkspaceAccessScreen.test.ts)
- app.language before config: PASS (TestSPAHandlerWithCustom_WritesBootPerRequest)
- Same-origin message: PASS (errors.test.ts)
- Read-only banner and hidden create: PASS (ReadOnlyBanner.test.ts, SpaceCreateMenu.test.ts, useInlineCreate.test.ts)
- Documented: PASS (docs/postgres-backend.md and its guide)

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug)
- [x] User-facing documentation updated (docs/postgres-backend.md and GUIDE-postgres-backend)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] PR created with `gh pr create` after the bug is done; CI watched there
