---
id: IMPL-FBVDROX
type: implementation-checklist
title: 'Implementation: tenant: provision a new org — eager IdP webhook plus a single-flighted lazy backstop'
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

**Tests:** unit tests in `internal/tenanthost/provision_test.go`,
`statussync_test.go`, `internal/idporg/client_test.go`,
`internal/tenant/registry_test.go`, `cluster_test.go`; postgres integration
tests in `internal/tenant/tombstone_postgres_test.go`,
`idpsuspend_postgres_test.go`, `provision_postgres_test.go` and
`cmd/rela-server/tenantmode_postgres_test.go`
(`TestTenantMode_LazyProvisioning`, `TestTenantMode_CreateServeSuspendErase`).

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

**Verification Evidence:** Automated only. On the final commit 13cd61e18: `just test` passed
(all packages, plus the pre-commit hook's `go test -race ./...`), `just lint`
0 issues on every build tag, `just comment-lint` clean, `just arch-lint` OK,
`just plimsoll` OK, `just lint-md` 0 issues, `just coverage-check` PASS (total
85.8%). A security review by the rela-security-reviewer agent and an automated
reviewer found the issues recorded as RR-RQ2INPS, RR-G9KL9T5, RR-6EHB7NR,
RR-SM1A2S7 and RR-GA0WMBD; all were fixed on the branch. The later `/code-review` findings are recorded on the review checklist.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
