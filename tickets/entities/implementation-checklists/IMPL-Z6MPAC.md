---
id: IMPL-Z6MPAC
type: implementation-checklist
title: 'Implementation: Fail CI instead of skipping when the pgstore suite has no database'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] ~~Integration tests written~~ (N/A: this change IS test infrastructure —
the "integration test" is the pgstore conformance suite it protects, which
already exists)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Changes:

- `internal/store/pgstore/testdb_test.go` — new `requireDBEnv`
(`RELA_TEST_DATABASE_REQUIRED`) plus `missingDSN()` and a shared
`skipOrFailWithoutDSN()` helper. The helper is the load-bearing piece: it is the
single place that decides skip-vs-fail.
- `internal/store/pgstore/listener_test.go` — `dsnForSchema` routed through the
shared helper instead of its own env check.
- `.github/workflows/ci.yml` — Postgres Backend job sets
`RELA_TEST_DATABASE_REQUIRED: "1"`; the test step now runs with `-v`, tees to a
log, and fails on any `--- SKIP`.
- `justfile` — documents the new variable on `test-postgres`.

Edge case that mattered: **three** sites checked the env var independently
(`adminConn`, `testDSN`, `dsnForSchema`). The first version of the guard only
covered `adminConn`, so `TestDebugQueryTracer_FromPoolEmits` still skipped
silently under strict mode. Caught by running the negative case rather than
assuming; consolidating all three is most of the diff.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

The guard's own messages interpolate the env var *constants* rather than
repeating the literal strings, so a rename cannot leave stale guidance.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Three modes, against a live PostgreSQL 16:

| Mode | Command | Result |
|---|---|---|
| default | `go test -tags postgres ./internal/store/pgstore/` | clean skip, exit 0 |
| strict, no DSN | `RELA_TEST_DATABASE_REQUIRED=1 go test ...` | 264 failures, actionable message, **0 remaining skips** |
| strict + DSN | both env vars set, `-race` | full suite green (~30s) |

The middle row is the one that matters: it is the negative case, and running it
is what exposed the uncovered third skip site. Re-verified after consolidation —
`grep -c -- '--- SKIP'` went from 1 to 0.

Also simulated the exact CI step locally (`-v`, tee, grep guard) against the
live database: guard passes with zero skips.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — the whole point: three duplicated env
checks collapsed into one helper. Leaving them separate is precisely how the
first version of this guard was incomplete.
- [x] No security issues introduced — the workflow change adds a static literal
env var, no untrusted input reaches any `run:` block
- [x] No silent failures (errors logged AND returned) — this ticket exists
*because* of a silent failure; the guard's messages name the variable to set and
the variable to unset
- [x] No debug code left behind

`golangci-lint` 0 issues (one `perfsprint` hit fixed); `gofmt` clean; `go vet
-tags postgres` clean.
