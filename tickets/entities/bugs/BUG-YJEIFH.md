---
id: BUG-YJEIFH
type: bug
title: Postgres-backed e2e specs fail to start rela-server-postgres when a database IS configured
description: history-url-params.spec.ts (6 tests) and relation-history.spec.ts (2 tests) fail with 'spawn bin/rela-server-postgres ENOENT' or a serverUrl setup timeout whenever RELA_E2E_DATABASE_URL is set. Reproduced identically on unmodified develop, so not caused by TKT-YOED3R. Hidden in most environments because the specs SKIP when the env var is unset.
priority: medium
status: backlog
---

## Symptom

The two postgres-gated e2e specs fail every run on a machine (or CI job) where
`RELA_E2E_DATABASE_URL` is set:

- `e2e/tests/history-url-params.spec.ts` (6 tests)
- `e2e/tests/relation-history.spec.ts` (2 tests)

Errors are one of:

```text
Error: spawn <root>/bin/rela-server-postgres ENOENT
Test timeout of 30000ms exceeded while setting up "serverUrl".
```

## Not caused by TKT-YOED3R

Found while landing PR #1444, and checked rather than assumed, because
`develop`'s own CI run was green:

- Built a worktree at `origin/develop` (`f5702e75`), built the same
`rela-server-postgres` binary and the e2e frontend bundle, ran
`history-url-params.spec.ts` against a local PostgreSQL.
- **6 failed on `develop`; 6 failed on the PR branch — identical.**
- `git diff origin/develop...HEAD -- e2e/` is empty on that branch.

## Why it hides

The fixture skips these specs when `RELA_E2E_DATABASE_URL` is unset
(`POSTGRES_E2E_ENABLED`), which is the default. So they SKIP in most
environments and only fail where a database is actually configured — the
opposite of the usual flaky-test shape, and why `develop` can look green.

## Notes for whoever picks this up

`ENOENT` is misleading: the binary exists at the path in the message and runs
fine when executed directly (`./bin/rela-server-postgres --help` exits 0). Node
reports `ENOENT` from `spawn` for causes other than a missing file, so the real
failure is likely in process startup or in `waitForServer`'s readiness probe
(`/api/v1/_config`), not in path resolution. `spawnServer` retries 3× and then
reports only the last error, which is why the surfaced message is unhelpful.

The fixture attaches the server's stdout/stderr to the Playwright report on
failure (`rela-server-postgres.log`) — read that first; it was not inspected
during triage.

## Acceptance

- The two specs pass with `RELA_E2E_DATABASE_URL` set, locally and in CI.
- The failure mode reports the server's actual startup error rather than
a bare `ENOENT`.
