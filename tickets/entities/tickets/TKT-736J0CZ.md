---
id: TKT-736J0CZ
type: ticket
title: 'neoq: stop the cron goroutine on queue shutdown before tenant eviction'
kind: chore
priority: medium
effort: s
started: "2026-11-07"
completed: "2026-11-07"
status: done
---

## Description

neoq starts a cron goroutine per backend and never stops it: `Shutdown` cancels
contexts but does not call `cron.Stop`. 20 create/close cycles leak 20
goroutines (`internal/jobs/neoqqueue.go:482`).

One long-lived queue per process makes this harmless today. In tenant mode the
registry opens and closes a tenant's services as tenants are evicted, and each
close leaks one goroutine for good.

## Approach

- Fix it on the `sourcehaven` branch of the sourcehaven-bv/neoq fork: stop the
cron scheduler in `Shutdown`. Offer the fix upstream.
- Bump the `replace` in `go.mod` and remove the KNOWN LEAK note.

## Acceptance criteria

1. A test opens and closes a postgres queue many times and asserts the
goroutine count returns to its baseline.
2. The schema-pinned DSN and cross-process shutdown tests still pass.
