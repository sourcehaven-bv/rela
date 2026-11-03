---
id: IMPL-YBVL7G
type: implementation-checklist
title: 'Implementation: Named dashboards: dashboards: in data-entry.yaml and dashboard: <name>'
started: "2026-10-15"
completed: "2026-10-15"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
  - `internal/dataentryconfig/dashboard_test.go`: YAML load and resolve, `DashboardRef` YAML/JSON round trip, validation error table (unknown name in nav, space home, page tab and `available_on`; reserved `default`; invalid name; card errors under `dashboards[<name>]`).
  - configedit `TestCheckEdit_NamedDashboards`; acl `TestDataEntryPermissions_NamedDashboardCards`; queryplan `TestStaticIndexSpecsIncludeNamedDashboardCards`.
- [x] Integration tests written (test full flow, not just units)
  - `internal/dataentry/dashboard_named_test.go`: `/api/v1/_dashboard?name=` through the handler (named, default, unknown 404 `dashboard_not_found`, card permission filtering per principal), sidebar, page tab and space home hrefs, `_config` wire shape, command scope, route words.
  - e2e `e2e/tests/named-dashboards.spec.ts` (4 tests): named page renders, space route `/s/support/dashboard/triage`, sidebar highlight, unknown name shows not-found.
- [x] Happy path implemented
- [x] Edge cases from planning handled
  - `dashboard: true` without a `dashboard:` block keeps the old behaviour; `dashboard: default` is an alias of `true`; unquoted yes/no is rejected; the name `default` is reserved under `dashboards:`; names must be URL segments.
- [x] Error handling in place (errors surfaced, not swallowed)
  - Unknown name is a config load error listing the known names; API returns a typed 404; SPA shows "Dashboard not found".

## Test Quality

- [x] Using fixture builders or factories for test data (existing `newTestApp`/YAML loaders, `card()` helper in Vitest)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
- e2e against a real server build (`just build-server-e2e`): named-dashboards, dashboard, spaces and configure specs all pass.
- `go test ./internal/dataentry/ ./internal/dataentryconfig/ ./internal/configedit/ ./internal/queryplan/ ./internal/cli/`: ok.
- Vitest: full run 302 files / 4223 tests passed before review fixes; affected dirs (views, configure, router) 32 files / 331 tests pass after.
- `golangci-lint`: 0 issues. `vue-tsc -b`: clean. `just docs` regenerated `docs/data-entry.md`.

## Quality

- [x] Code follows project patterns (check similar code): `DashboardRef` follows the `ActionConfirm` bool-or-string union; validation reuses `navTargetErrors`.
- [x] Checked for DRY opportunities: `DashboardByName`, `AllDashboards` and `dashboardRefError` are shared by the handler, acl, queryplan and validation; card checks share `validateDashboardCards`.
- [x] No security issues introduced: named dashboards use the same per-card permission filter as the default; configedit locks and protects `dashboards.*.cards.[].permission`.
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
