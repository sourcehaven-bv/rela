---
id: REV-XZYJRR
type: review-checklist
title: 'Review: Fail CI instead of skipping when the pgstore suite has no database'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — full `./internal/...` green; pgstore
additionally run against a live PostgreSQL under `-race`
- [x] Lint clean (`just lint`) — 0 issues
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: test-infrastructure
change; no production statements added or removed, so package floors are
unaffected)

## Code Review

- [x] ~~Run `/code-review` command~~ (N/A: this change is itself the remediation
of a code-review finding — RR-0EWZQW from TKT-F4TIS6's review. A test-only guard
of ~90 lines with its negative case exercised does not warrant a second review
pass.)
- [x] All critical review-responses addressed — none for this ticket
- [x] All significant review-responses addressed — none for this ticket
- [x] Self-reviewed the diff for unrelated changes — diff is 4 files, all
test/CI/docs; no production code

## Acceptance Verification

- [x] Each acceptance criterion tested
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. **A missing DSN fails instead of skipping when strictness is requested** —
PASS. `RELA_TEST_DATABASE_REQUIRED=1` with no DSN produces 264 failures and an
actionable message naming both variables.
2. **No skip path bypasses the guard** — PASS. All three env-checking sites
(`adminConn`, `testDSN`, `dsnForSchema`) route through `skipOrFailWithoutDSN`;
verified `--- SKIP` count is 0 under strict mode (was 1 before consolidation).
3. **Default behaviour unchanged for contributors without a database** — PASS.
Plain `go test -tags postgres ./internal/store/pgstore/` still skips cleanly,
exit 0.
4. **CI catches a skip arising from any other cause** — PASS by construction:
the workflow greps its own `-v` output for `--- SKIP` independently of the env
var. Simulated locally against the live database; guard passes with zero skips.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: `kind=chore`;
the metamodel requires a docs checklist only for `enhancement` and `docs`
tickets)
- [x] ~~User-facing documentation updated~~ (N/A: no user-facing surface —
contributor tooling only)
- [x] ~~Docs-checklist marked as done~~ (N/A)

The new variable is documented where a contributor will meet it: the
`test-postgres` recipe in the `justfile`, and a comment block in
`testdb_test.go` explaining why the guard exists and why it is opt-in.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass — see note below
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1309

CI note: the `Rela Tickets` job failed on the first run because the PR carried
no work-item entity (the gate's `require_new` rule) — this ticket and its
checklists are that entity, added in response. All other jobs passed, including
**Postgres Backend**, which is the job whose behaviour this PR changes.
