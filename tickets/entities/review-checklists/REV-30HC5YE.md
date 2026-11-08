---
id: REV-30HC5YE
type: review-checklist
title: 'Review: rela-server: multi-tenant host that routes a verified org_id to its tenant''s App'
started: "2026-11-08"
completed: "2026-11-08"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (go test -race on changed packages, default and postgres tags; full suite runs in CI)
- [x] Lint clean (`just lint`) (golangci-lint on changed packages, default and postgres tags)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) (tenanthost 94.9%, tenant 89.0%, cmd/rela-server 54.0%; all above the floor)

**Comment findings.** No new advisory findings introduced.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (cranky-code-reviewer and rela-security-reviewer)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** Design: RR-ZIKLZ02, RR-Q6K62YV, RR-42CM1YD, RR-4R3J2XM,
RR-3P2TTKY, RR-NJ7IG1V, RR-3WPKYPW, RR-BJWUKKX. Code: 19 linked via
has-review-response; 3 minor deferred to TKT-N3X235N, 1 minor and 1 nit wont-fix
with reasons.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- Routes by verified org_id: PASS (TestHost_RoutesByVerifiedOrg, TestTenantMode_TenantsCannotSeeEachOther)
- Fail closed on no/unknown org and open errors: PASS (TestHost_FailsClosed, TestTenantMode_FailsClosedOverRealTenants)
- Per-tenant audit sink (TKT-SNVR8UV): PASS (TestTenantIsolation_OwnAuditAndScriptEngine)
- Single-tenant mode unchanged: PASS (TestTenantsPath, existing cmd/rela-server suite)
- No DSN on command line or in logs: PASS (env fallback, password refusal TestLoadConfig_RefusesPasswords)

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-UICMKZY

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (done by the PR step after the ticket is done)
