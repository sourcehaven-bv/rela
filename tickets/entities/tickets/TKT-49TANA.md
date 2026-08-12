---
id: TKT-49TANA
type: ticket
title: Fail CI instead of skipping when the pgstore conformance suite has no database
kind: chore
priority: high
effort: s
status: done
---

## Problem

The pgstore conformance suite skips itself when `RELA_TEST_DATABASE_URL` is
unset, and **a skip is indistinguishable from a pass** in the `go test` exit
code and in the CI summary.

That suite is the *only* mechanism enforcing the backend-parity rule in the root
`CLAUDE.md` ("the default build must answer identically to postgres"). So a
dropped env var, a renamed service container, or a workflow copied without its
env block silently turns the parity gate into a no-op — and fs/mem-vs-postgres
divergence merges with nothing going red.

Not hypothetical. Two real defects shipped past every non-DB check while
implementing TKT-F4TIS6 (`store.GraphQuery` property predicates), caught only by
standing up a live database by hand:

- **`42P18`** ("could not determine data type of parameter") on every
any-endpoint query — an unreferenced positional placeholder.
- **jsonb-vs-Go disagreement on list-valued properties**, which *inverted*
`is empty` and made multi-select equality match nothing on postgres while
matching correctly on fsstore.

Found during code review of TKT-F4TIS6 and split out as its own change, because
it guards every future store change rather than that one ticket.

## Fix

Two independent layers, because they fail differently.

1. **`RELA_TEST_DATABASE_REQUIRED` (Go side).** When set, the absent-DSN skip
becomes a hard failure with an actionable message. CI's Postgres Backend job
sets it.

All skip sites route through one shared `skipOrFailWithoutDSN` helper. There
were **three** independent env checks — `adminConn`, `testDSN`, and
`listener_test.go`'s `dsnForSchema` — so the first version of the guard left a
path uncovered and a test still skipped silently under strict mode.
Consolidating them is most of the diff, and is what makes the guard total.

2. **A `--- SKIP` grep on the CI test output.** Blunter, and catches what the env
var cannot: a suite that skips for some *other* reason — a future build tag, a
`t.Skip` added inside a subtest, a conformance case quietly excluded.

## Why opt-in rather than "strict whenever CI is set"

Forks and Dependabot PRs run the same workflow, and a contributor running the
suite locally without a database must still get a clean skip. Strictness is
promised by the environment that provides a database, not inferred from CI.

## Verification

All three modes exercised against a live PostgreSQL 16:

| Mode | Result |
|---|---|
| default (no flags) | clean skip, exit 0 |
| strict, no DSN | 264 failures, actionable message, **0 remaining skips** |
| strict + DSN | full suite green under `-race` |

`golangci-lint` 0 issues; `gofmt` clean; `go vet -tags postgres` clean.

Test-only change plus workflow/justfile — no production code touched.

## PR

https://github.com/sourcehaven-bv/rela/pull/1309
