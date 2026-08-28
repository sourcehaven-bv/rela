---
id: TKT-TNT9RS
type: ticket
title: 'tenant: resolve a verified org to a store, fail closed on an unknown tenant'
kind: enhancement
priority: high
effort: l
tags: [security]
status: done
---

## Description

Multi-tenant SaaS (RES-D54281) turns on one lookup: a verified `org_id` must
select **which store a request may touch**. Nothing does that today. `OrgID` is
carried, forge-proof, all the way to the ACL (`principal.go:185`,
`router.go:408`) and is then **used for nothing but recording**
(`docs/acl-security.md`, `principal.go` — "recorded, not enforced").

Build the resolution spine: `org_id -> DSN -> *appbuild.Services`, with a
bounded resident set, and **fail closed** when the org does not resolve.

This is the piece RES-D54281 calls "the spine of the work". The four
prerequisites it named have all shipped — explicit DSN (TKT-1J5KEV, #1318),
shared-base/per-store split (TKT-P938T7), one shared NOTIFY channel
(TKT-9TOEBH), Postgres-backed `state.KV` (TKT-VC27L3) — so `SharedBase.Assemble`
is already documented as "a multi-store host calls it directly, once per store".
This ticket is the caller that documentation was written for.

**This ticket does not provision tenants and does not erase them.** See
TKT-TNPRV8 and TKT-TNERAS.

### Why isolation is not an ACL rule

`internal/acl` is union semantics with no deny primitive, so "deny unless org
matches" is inexpressible as a role rule — RES-D54281 establishes this and
`TestAssertedRoles_OrgIsNotEvaluated` pins the absence.

Isolation therefore comes from **the store handle a request can reach**. A
request resolved to tenant T holds a `*Services` whose store is a pool pinned to
schema T by `search_path`. A cross-tenant read is not a missing `WHERE` clause
that a reviewer must spot; it is `relation "entities" does not exist`, enforced
by PostgreSQL. That is the whole reason RES-D54281 chose schema-per-tenant over
a `tenant_id` column: the column form makes every one of ~20 indexes and every
query a place where one missed predicate is a silent leak **with no compile
error**.

## Decision: where the tenant map lives

RES-D54281 decided the map is **rela-owned, not a JWT claim and not Pratique**,
and left storage open. **Decided here: an operator-authored config file,
`tenants.yaml`, loaded at boot, behind a one-method `Resolver` interface.**

Reasoning, in the order that decided it:

1. **It cannot live in the thing it shards.** RES-D54281 names the recursion
   directly: the map "cannot live inside the thing it shards". A table in a
   tenant schema is circular by construction, and a table in a *control* schema
   means the process needs a DSN to find DSNs — so there is always a
   bootstrap-config layer underneath. A config file **is** that layer. Putting
   the map in a database does not remove the file; it adds a database *and*
   keeps the file.
2. **The operator already authors exactly one config.** That is the premise of
   the whole design — one metamodel, one `acl.yaml`, one `data-entry.yaml`. A
   `tenants.yaml` beside them is the shape the operator is already in, and it is
   reviewable, diffable, and deployable by the same path as the rest.
3. **The interface is the real decision; the backing store is not.** RES-D54281
   says the requirement is that "the lookup must be a single seam: one
   interface, one cache with explicit invalidation. Keep that true and 'where
   the map lives' stays a config change." A file satisfies the seam at the
   lowest cost, and a DB-backed `Resolver` is a drop-in the day self-serve
   signup needs writes at runtime.
4. **A file is honest about the current scale.** There is no self-serve signup
   yet (TKT-TNPRV8 is not built), so today every tenant arrives through an
   operator action anyway. A control-schema table would be infrastructure built
   for a provisioning flow that does not exist, and it would have to be designed
   *before* the thing that writes to it — the wrong order.

**What this deliberately does not decide:** the day provisioning lands
(TKT-TNPRV8), the resolver behind the interface likely becomes DB-backed so a
webhook can write a tenant without a deploy. That is a swap of one
implementation, which is precisely the property being bought here.

**Rejected: DSN in a JWT claim.** Already rejected by RES-D54281, restated here
because it is the tempting shortcut. Storage topology in an identity token means
Pratique knows about database clusters and every rebalance becomes a Pratique
deploy; it also breaks `jwtauth`'s deliberate provider-agnosticism.

## Decision: the connection ceiling and the resident set

RES-S8CH9C measured it and RES-D54281 reuses the number: `pgstore.Open` builds a
pool with **no `MaxConns` set** (defaults to `max(4, numCPU)`), plus **one
dedicated non-pooled connection for `LISTEN`** (`open.go:57`), plus the sweep.
On a 16-core box that is **~17 connections per open store**. Against a typical
`max_connections=100` that is ~5 tenants resident, not 500.

TKT-9TOEBH already removed the per-tenant LISTEN connection at the *channel*
level (one `rela_changed` channel for every schema), but a resolver that calls
`pgstore.Open` per tenant still gets a pool and a listener per tenant, because
`Open` owns that construction.

So the ceiling is **not** something this ticket can delete; it is something this
ticket must **refuse to exceed**. Two rules follow, and they are the design:

1. **The resident set is bounded, and the bound is a number the operator sets** —
   not the tenant count. RES-D54281's horizon requires "live-project count
   bounded independently of total tenant count", and that is the only property
   that makes 500 tenants on one process a question about latency rather than
   about `max_connections`.
2. **Eviction closes the store.** `Services.Close()` is already per-store by
   construction (TKT-P938T7 pinned it: it "tears down the store and search
   closer it was assembled with, never anything belonging to the base"), which
   is exactly what makes eviction safe. Evicting tenant A must leave the shared
   base and every other resident tenant untouched.

Apartment-style pooling — one shared pool with `SET search_path` per checkout —
is the eventual answer and RES-D54281 describes it. It is **out of scope here**
because it collides with the version sweep, which deliberately runs a whole tick
on one acquired connection under a session-scoped advisory lock and whose doc
warns that issuing inserts via the pool would "silently void the single-writer
guarantee". Rotating `search_path` on a shared pool breaks that. Bounding the
resident set gets the same headroom without touching the sweep's invariant.

## Decision: fail closed

The security requirement is one sentence — **no cross-tenant data leaks** — and
every ambiguous case resolves the same way: **no tenant, no store, no request.**

Specifically, all of these are denials, not fallbacks:

- No `org_id` on the principal (unverified, or a verified principal from an
  issuer that does not assert one).
- An `org_id` that is not in the map.
- A tenant entry that fails validation.

The trap being avoided is the one RES-D54281 catalogues under "known fail-open
traps": a zero-value `ReadQueryResult` **aliases `AllowAll`**, and
`nopReadGate.HoldsPermission` returns `true` unconditionally. A resolver that
returned a zero-valued or default store on an unknown org would join that list
as the worst member — it would not widen permissions within a tenant, it would
hand one tenant another tenant's database. **A missing tenant must be an error
value, never a zero value.**

## Scope

**In scope**

- A `tenant` package: `Tenant` (org ID + DSN + schema), `Resolver` (one method:
  org ID -> `Tenant`), and a config-file-backed implementation.
- Strict schema-name validation. RES-D54281 flags schema names as "a trust
  boundary needing strict validation (`^[a-z][a-z0-9_]{0,30}$`)" — an
  operator-authored file is inside the trust boundary but a typo must still fail
  loudly at load, not at first `CREATE`.
- A registry that resolves an org to an assembled `*appbuild.Services` over one
  shared `*appbuild.SharedBase`, with a bounded resident set and eviction that
  closes the evicted store only.
- Duplicate detection at load: two orgs mapping to one schema is a
  cross-tenant leak spelled as a config typo, and must fail at boot.
- Tests, on the postgres build, proving the isolation property against a real
  database: two tenants, two schemas, one process; data written as tenant A is
  **not readable** as tenant B.

**Out of scope**

- **Provisioning** — webhook, lazy backstop, single-flight, 202+poll (TKT-TNPRV8).
  This ticket resolves tenants that already exist and fails closed on ones that
  do not; it never runs `CREATE SCHEMA`.
- **Erasure** — `DROP SCHEMA CASCADE`, retention SLA, the audit-log decision
  (TKT-TNERAS).
- **Mounting the registry in `rela-server`'s HTTP path.** `dataentry.App` is one
  App per store and sits at `//plimsoll:max-methods=104`; an App-per-tenant host
  is its own ticket and its own review. RES-D54281 is explicit: "Do **not** put
  per-request tenant logic on `App`."
- Apartment-style shared pooling, and cluster sharding.
- The non-HTTP entry points (scheduler, CLI, MCP), which build principals from
  plain literals and have no org. RES-D54281 flags deciding what a CLI write
  means in a SaaS deployment; that decision is not this ticket's.

## Load-bearing constraints

- **The shared base is shared and must not be mutated.** `meta` and `aclPolicy`
  are pointers handed to every tenant. TKT-P938T7 pinned this with
  `TestSharedBase_AssemblyDoesNotMutateSharedValues`; a registry assembling N
  Services makes a violation a cross-tenant defect rather than a curiosity.
- **Closing one tenant must not disturb another.** Already true of
  `Services.Close()`; the registry must not add anything that breaks it.
- **Constructors reject nil required fields** (CLAUDE.md). A registry without a
  base or without a resolver must fail at construction.
- **The DSN must never reach a command line** (TKT-1J5KEV's constraint). A
  tenants file read in Go code is fine; a `--tenant-dsn` flag is not.

## Acceptance criteria

1. A `Resolver` maps a verified org ID to a tenant record, and returns a
   distinguishable "unknown tenant" error — not a zero value — for anything it
   cannot resolve.
2. An unknown, empty, or invalid org yields **no store**. Pinned by a test that
   asserts the error, and asserts no `*Services` is returned.
3. A registry assembles per-tenant `Services` over one shared base, and the
   metamodel and ACL policy are parsed once regardless of tenant count.
4. The resident set is bounded by an operator-set limit; exceeding it evicts,
   and eviction closes the evicted store and nothing else. Pinned by a test that
   evicts one tenant and keeps using another.
5. Two orgs mapping to the same schema fails at load, not at first request.
6. Schema names are validated against `^[a-z][a-z0-9_]{0,30}$` at load.
7. **On the postgres build, against a live database: an entity written through
   tenant A's store is not visible through tenant B's store.** This is the
   ticket's centrepiece, and it must be an executed assertion, not prose.
8. `just test`, `just lint`, `just arch-lint`, `just plimsoll`, and
   `just comment-lint` pass.

## Notes

- The test harness for criterion 7 already exists and is routine:
  `internal/appbuild/backendtest/backendtest_postgres.go:88-116` creates a
  private schema per test and hands back a DSN pinned to it via
  `dsnWithSearchPath`, which re-serializes the parsed config rather than doing
  query-string surgery (pgx accepts both URL and key/value DSNs, and appending
  `?search_path=` silently corrupts the latter). Reuse that construction rather
  than inventing a second one.
- RES-D54281's load-bearing claim — **`pgstore` needs zero changes** — is
  re-verified and still holds: schema is purely the connection's `search_path`,
  and `current_schema()` derives the NOTIFY payload's schema field and every
  advisory-lock key. This ticket adds no pgstore code.
