---
id: REV-813R6RS
type: review-checklist
title: 'Review: tenant: derive tenant location from org_id; remove tenants.yaml'
started: "2026-11-10"
completed: "2026-11-10"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): `go test -race` on every changed package (tenant, cli, rela-server, rela-docs, tenanthost, dataentry), default and postgres tags, against a schema-pinned test database; the full suite runs in CI
- [x] Lint clean (`just lint`): golangci-lint v2.11.4 (the CI version) on the changed packages, default and postgres tags, 0 issues
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`): tenant 73%, cli 65%, tenanthost 94%, all above their floors

**Comment findings.** `just comment-report` lists the advisory rules
(duplication, nil-contract, param-contract, restatement). They are not a merge
gate, but a finding your diff *introduces* should be fixed or suppressed — don't
grow the backlog.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-EXOZLN9, RR-HVUL13Y (significant, addressed);
RR-LVV4DHN, RR-X6ECA18, RR-6SX6HC8, RR-DLD32N3, RR-NFBNM21, RR-ET8ZLU3,
RR-H0L9XNQ, RR-962E28H, RR-VOQD1PF (minor, addressed); RR-67UYXV7 (nit,
addressed); RR-7BYPVLL (nit, wont-fix). Also arch-lint, plimsoll and archplan ci
against the merge base are clean.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. Two orgs resolve to different schemas: PASS (TestSchemaFor, TestTenantMode_CreateServeSuspendErase).
2. Unknown, suspended, erasing refused: PASS (TestClusterResolver_Resolve, TestProvisioner_Lifecycle, TestTenantSelect_RefusesUnservedTenants).
3. Collision refused: PASS (TestClusterResolver_Collision, TestProvisioner_CollisionRefused).
4. Create, serve, erase end to end on postgres: PASS (TestTenantMode_CreateServeSuspendErase, manual run of rela-server-postgres).
5. Nothing reads tenants.yaml: PASS (TestNothingReadsTheTenantMap, TestRefuseLegacyMap).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-CUN7DMF

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
