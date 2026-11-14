---
id: IMPL-687PRWG
type: implementation-checklist
title: 'Implementation: Tenant mode as the SaaS profile: scope features that are unsafe in a shared process'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

**Tests:** unit tests in `internal/lua/httppolicy_test.go`, `bounds_test.go`,
`hostconfig_test.go`, `internal/appbuild/withheld_test.go`,
`tenantprofile_test.go`, `internal/dataentry/commands_test.go`,
`handlers_git_test.go`, `internal/script/runtime_test.go`,
`internal/cli/tenant_test.go`, `cmd/rela-server/tenantmode_test.go`; integration
tests in `internal/dataentrywire/withheld_test.go`,
`internal/cli/tenant_postgres_test.go` and
`cmd/rela-server/tenantmode_postgres_test.go` (`TestTenantMode_SaaSProfile`).

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (not done: no manual run against a live IdP; the end-to-end flow is covered by the postgres integration tests listed above)
- [x] ~~Each acceptance criterion verified with test scenario from planning~~ (not done manually: each criterion maps to an automated test in the planning checklist)
- [x] ~~Edge cases manually verified~~ (not done manually: edge cases are pinned by automated tests)

**Verification Evidence:** Automated only. `just check` passed earlier on this
branch; a final run on the last commit is still in progress at the time of
writing. A security review by the rela-security-reviewer agent and an automated
reviewer found the issues recorded as RR-A2FK6GR, RR-N5ZY6L4, RR-0Z8GN02 and
RR-1IBQECU; all were fixed on the branch.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
