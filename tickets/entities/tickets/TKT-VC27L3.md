---
id: TKT-VC27L3
type: ticket
title: Postgres-backed state.KV so render cache, settings and scheduler state survive a cluster (and tenants)
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

`state.KV` is filesystem-backed and rooted at the project's `.rela/`
(`buildStateKV`, `appbuild.go:1010`). Everything on it — the document render
cache, user settings, the logo/theme, and scheduler bookkeeping — is therefore
**node-local**, which is wrong for a load-balanced postgres deployment and
actively unsafe if one process ever serves several tenants.

Add a **PostgreSQL-backed `state.KV`** implementation, selected by the postgres
build, alongside the existing `FSKV`.

### Why (stands on its own, independent of multi-tenancy)

`docs/postgres-backend.md` already documents running **several `rela-server`
instances behind a load balancer** against one database. Under that topology,
today:

- The **document render cache is per-node.** N nodes means up to N renders of
the same document and an inconsistent hit rate; a `command:` renderer shelling
out to an external tool is exactly the expensive thing worth caching once.
- **User settings and the logo/theme are per-node.** An operator uploads a logo,
it lands on whichever node served the POST, and other nodes keep serving the old
one until they happen to be hit. That is a visible, confusing bug with no error
message.
- **Scheduler state is per-node** — mitigated today only because operators run
one scheduler.

PostgreSQL is already the source of truth for entities, relations, attachments
and search in that deployment. State that must be consistent across nodes
belongs in the same place. **No multi-tenancy needed to justify any of this.**

### Why it also matters later

Multi-tenant SaaS (RES-D54281) puts N tenants in one process, all sharing one
`.rela/`. Then the same three surfaces collide across tenants — see the
collision analysis below. A keyspace that is already per-tenant in the DB makes
that a non-issue rather than a second project.

## The seam already exists — do not invent one

`state.KV` (`internal/state/state.go:20-34`) is **already** the swap boundary,
with three methods (`Get`/`Put`/`Delete`, `[]byte` values, `/`-separated
hierarchical keys) and a package doc that says so outright: *"The KV interface
is the swap boundary. FSKV is the default backend; callers can plug in Redis,
DynamoDB, etc. by implementing KV."*

So this ticket is **an implementation, not a refactor**. Do not add a repository
layer, a cache abstraction, or a second interface — CLAUDE.md's "no repository
or transaction abstractions" rule applies. Consumers keep taking `state.KV`.

## Consumers (all already on the interface)

| Consumer | Keys | Site |
|---|---|---|
| Document render cache | `documents/<entryID>-<contentHash>.html` | `document.go:157,183,364` |
| User settings | `userDefaultsFile` | `settings_service.go:61,81` |
| Logo / theme | `userLogoFile`, `userLogoExtFile` | `logo_store.go:130,138,167` |
| Scheduler | `scheduler-state.json` | `scheduler.go:241,255` |

Note `dataentry/app.go:615` roots a **second** `RootedFS` KV; it must be swapped
too or half the consumers stay on disk.

## Scope

**In scope**

- A `state_kv` table (key TEXT PK, value BYTEA, updated_at) + migration, and a
`pgstore`-side (or `internal/state`) implementation satisfying `state.KV`.
- Wire it in the postgres recipe (`appbuild_postgres.go`); FS/memory builds keep
`FSKV` unchanged.
- A conformance test both implementations pass — `FSKV` has real semantics worth
pinning (missing key returns an `os.IsNotExist`-satisfying error; `Delete` of a
missing key is NOT an error, `state.go:29-32`). A backend that gets either wrong
breaks `GetCached`'s "missing ⇒ re-render" path silently.

**Out of scope**

- Per-tenant keyspacing. That belongs with the appbuild split (RES-D54281); this
ticket makes it *possible* by moving the data somewhere a tenant prefix or a
per-tenant schema can apply. Landing this first means that work is wiring.
- The audit log. Deliberately excluded — it is **operator-focused**, tenants have
no disk access, and its GDPR position is the same as backups (retention expiry
is the erasure SLA). Decided 2026-08-14; do not fold it in.
- Changing any consumer's keys or semantics.

## Load-bearing details

- **Value size.** The logo is arbitrary user-uploaded bytes and rendered HTML can
be large. `BYTEA` is fine (1GB limit) but the row is read/written whole — same
pattern as `attachments` (`pgstore/attachment.go:57`), so follow it, including a
sane cap.
- **The render cache is a cache, not state.** A DB miss or error must degrade to
a re-render, never a 500. `GetCached` already returns nil on any error
(`document.go:145-146`) — preserve that.
- **Don't put cache writes in a `Tx`.** CLAUDE.md forbids slow external I/O in a
`Tx` callback, and a render is slow by definition. Write after the render, on
the pool.
- **`Delete` semantics.** Deleting a missing key must stay non-error, or
`logoStore.Delete`'s documented idempotence (`logo_store.go:100-102`) breaks.

## Collision analysis this closes (for the multi-tenant case)

Verified at commit e7003d74:

1. **Document cache — could serve one tenant's HTML to another.** The key is
`documents/<entryID>-<contentHash>.html` with **no tenant component**, and
`contentHash` is FNV-64a over the *entry entity's* id/type/properties/content
only (`computeDocumentHash`/`hashEntities`, `document.go:492-503`) — not over
the rendered output. Two tenants with an identically-named entity of identical
content share a file, and whoever renders first serves their HTML to the other.
A Lua `script:` renderer can read well beyond the entry entity, so identical
entry content does **not** imply identical output. Narrowed (not removed) by
only `command:` renders using the disk cache (`document.go:113-116`).
2. **Scheduler** — one `scheduler-state.json`; tenant B's run would suppress
tenant A's next run.
3. **Settings / logo** — fixed key names; last writer wins across tenants.

## Acceptance criteria

1. A `state.KV` implementation backed by PostgreSQL, selected on the postgres
build; FS/memory builds are byte-for-byte unchanged.
2. Both implementations pass one shared conformance suite covering: round-trip,
overwrite, hierarchical keys, missing-key error satisfies `os.IsNotExist`,
`Delete` of a missing key succeeds, `Delete` then `Get` reports missing.
3. Both KV construction sites (`appbuild.go:1010`, `dataentry/app.go:615`) use
the selected backend — no consumer left on disk.
4. Two `rela-server` processes against one schema observe each other's settings,
logo, and render cache. This is the property the ticket exists for and needs a
real two-process (or two-`Services`) test, not a single-process assertion.
5. A KV failure on the render path degrades to a re-render, not an error response.
6. `just test-postgres` and `just ci` pass.

## Notes

- `nopKV` (`appbuild.go:1021`) is the no-cache-dir fallback; the postgres path
should not need it, but keep it for FS builds.
- Consider whether the render cache wants a TTL/eviction story once it is shared
— on disk it grew unbounded per node, which was survivable; one shared table
across a fleet grows faster. Not required for this ticket, but note it rather
than discover it.
