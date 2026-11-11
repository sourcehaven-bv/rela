---
id: TKT-TMXHBZW
type: ticket
title: Replace neoq with River on postgres and an in-house memory queue
kind: chore
priority: medium
effort: l
started: "2026-11-10"
completed: "2026-11-10"
status: done
---

## Description

neoq is close to unmaintained. Rela runs it from a fork with four patches (see
the `replace` comment in `go.mod`). Two of the patches were never proposed
upstream. The schema-pinned DSN fix (acaloiaro/neoq#149) has been open for
months. We also work around neoq gaps: a second pgx pool because neoq cannot
take ours, and our own retention sweep because neoq never deletes finished jobs.

Replace neoq with River (github.com/riverqueue/river) on the postgres tier.
Replace neoq's memory backend with a small queue of our own on the fs/desktop
tier. Do not absorb the neoq code.

## Approach

Two steps, each its own PR:

1. **Memory tier.** Write a channel-based in-process queue for `jobs.NewMemoryQueue`. It stays ephemeral by design (see the package doc). It must pass `jobstest`.
2. **Postgres tier.** Implement `jobs.NewPostgresQueue` on River:
   - Inject rela's pgx pool. Drop the second pool and `ensurePoolFloor`.
   - Set `Config.Schema` so the queue is scoped to the tenant schema.
   - Map `Job.IdempotencyKey` to unique jobs, and `Retry` intents to River retry policy and max attempts, keeping mechanism in `retry.go`.
   - Use River's job cleaner. Drop the retention sweep in `pgqueue.go`.
   - Create River tables per tenant schema with `rivermigrate`, wired into the existing tenant setup.
   - Plan the cutover of pending `neoq_jobs` rows: drain, or copy once. Drop the neoq tables afterwards.
   - Remove the neoq `replace` and dependency.

Out of scope: enqueueing inside the store transaction with `InsertTx`. It could
replace the deferred `Collector` on postgres, but it needs a seam through
`store.Tx`. File it separately if wanted.

## Acceptance

- `jobstest` passes on both tiers, including the stall and pool tests.
- Delivery stays at least once. A CLI command's shutdown does not stop rela-server's job processing (BUG-YAMD6J).
- No neoq import remains.
- `docs/postgres-backend.md` describes the River tables and migration.
