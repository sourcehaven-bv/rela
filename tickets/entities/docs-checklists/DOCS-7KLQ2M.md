---
id: DOCS-7KLQ2M
type: docs-checklist
title: 'Docs: Keyed lock seam (internal/lock): named mutual exclusion with per-tier backends'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Exported functions/types have godoc
- [x] Non-obvious decisions explained in comments
- [x] Package docs updated if package purpose changed

The package doc on `internal/lock` carries the design rationale, because this
seam's contract is the thing a future caller can get wrong. It states why the
lock is KEYED rather than a plain mutex (a backend that ignored the key would
be a correct-looking implementation of a useless contract, which is why
`locktest` asserts that distinct keys do not contend), and it scopes the seam
explicitly as mutual exclusion and NOT consensus — no fencing tokens, no
leases, no liveness guarantee.

`Locker.Acquire` documents the two decisions a reader would otherwise reverse:

- **Blocking, not try-or-skip.** Every pre-existing advisory-lock caller in
`pgstore` uses `pg_try_advisory_lock` and skips when another holder has it,
which is right for a reconciliation sweep and wrong for a request path, where
skipping silently drops the caller's work. Bounding the wait is the caller's
job via a ctx deadline.
- **Release is idempotent** and safe to `defer`.

Two further notes sit where the mistake would be made rather than in prose
elsewhere: `pgstore`'s advisory-lock key must cast to `int`, not `bigint`, and
the N-holder pool ceiling is recorded on the keyed-lock path because each held
lock pins a connection from the pool.

`ValidateKey`'s godoc records that it is strictly weaker than
`state.ValidateKey` (RR-1TQPWE), so a reader cannot assume the two are
interchangeable.

## Project Documentation

- [x] ~~CLAUDE.md updated with new patterns~~ (N/A: introduces no new
architectural pattern. A per-tier backend seam chosen at the wiring site is the
established shape already used by `store.Store`, `state.KV`, `jobs.Queue` and
`userstate.Store`.)
- [x] ~~docs/ updated for changed behaviour~~ (N/A: no observable behaviour
changes — nothing in the tree calls the seam yet. Verified: the only importers
of `internal/lock` are its own tests, `locktest`, and
`internal/store/pgstore/keyedlock.go`.)
- [x] ~~Architecture docs updated~~ (N/A: the new component is declared in
`.go-arch-lint.yml` and `arch-lint` is clean; no existing package boundary,
dependency direction or wiring contract changed.)

## External Documentation

- [x] ~~README updated~~ (N/A: no user-visible feature.)
- [x] ~~CLI reference updated~~ (N/A: no new or changed command or flag.)
- [x] ~~API docs updated~~ (N/A: no HTTP or MCP surface change.)

## Rationale for N/A

This ticket adds a seam and its backends, not a feature. There is no config
key, CLI flag, HTTP or MCP endpoint, schema or metamodel change, and no
production caller — `internal/lock` is imported only by its own tests and by
the `pgstore` adapter that implements it. An operator cannot observe the
change, so there is nothing for `docs/` to describe that would not go stale
before it became true.

The contract is documented where it is enforced: as godoc on the interface,
and as the `locktest` conformance suite, which is the executable form of the
same statements. That suite is what a second backend is held to, so it cannot
drift from the prose the way a separate document could.

User-facing documentation lands with the first consumer, TKT-1EM4KL
(declarative webhook routes), which is the ticket this seam was extracted from
and where the keyed lock becomes observable — that is the point at which an
operator can see contention behaviour and needs it described.
