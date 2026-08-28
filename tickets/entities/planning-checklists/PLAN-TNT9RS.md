---
id: PLAN-TNT9RS
type: planning-checklist
title: 'Planning: tenant resolution spine — org_id -> DSN -> Services, fail closed'
status: done
---

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined
- [x] Acceptance criteria documented

**Problem.** RES-D54281 chose schema-per-tenant and named the resolution seam
"the spine of the work". Every prerequisite it listed has shipped, and nothing
consumes them: `NewSharedBase` and `SharedBase.Assemble` are exported and
documented for a multi-store host, and have **zero production callers**.
`principal.OrgID()` is carried, forge-proof, all the way to the ACL and is used
only for audit attribution — `internal/acl` contains zero references to it.

**Scope (in):**
- New `internal/tenant` package: `Tenant`, `Resolver`, `Registry`.
- Config-file-backed resolver (`tenants.yaml`), validated at load.
- `Registry` assembling per-tenant `*appbuild.Services` over one `*SharedBase`,
  with a bounded resident set and eviction that closes only the evicted store.
- Fail-closed resolution: unknown/empty/invalid org returns an error and no store.
- Postgres-build isolation test against a real database.

**Scope (out):**
- Provisioning (TKT-TNPRV8) and erasure (TKT-TNERAS) — filed, not built.
- Mounting the registry in `rela-server`'s HTTP path. `dataentry.App` is
  per-store and at `//plimsoll:max-methods=104`; RES-D54281 says explicitly not
  to put per-request tenant logic on `App`.
- Apartment-style shared pooling; cluster sharding.
- Non-HTTP entry points (scheduler, CLI, MCP).

**Acceptance Criteria:** see TKT-TNT9RS. The load-bearing one is #7 — an entity
written through tenant A's store is not visible through tenant B's store, proven
by an executed test against a live PostgreSQL.

## Research

- [x] **Re-verified against develop `11c02c1b`, not against ticket statuses.**
  The four prerequisite tickets all read `status: backlog` while their code is
  merged, so every claim below was checked in code.
- [x] `WithDatabaseURL` / `resolveDatabaseURL` exist (`appbuild.go:856,868`).
  **Important asymmetry:** `appbuild.New` reads `Config.DatabaseURL` and
  *ignores* `WithDatabaseURL`; only `Discover` consumes the option. A registry
  must therefore set `Config.DatabaseURL` per tenant, not pass the option.
- [x] `SharedBase` exists (`appbuild.go:1141`); all three backends take
  `*SharedBase` (fs:48, postgres:47, memory:39).
- [x] `feedChannel = "rela_changed"` is a single shared constant
  (`pgstore/feed.go:55`), schema carried in the payload and derived from
  `current_schema()`.
- [x] `pgstore/statekv.go` exists, wired through `stateKVFor`
  (`versionsweep_postgres.go:158`) into `assemble`.
- [x] Nothing maps an org to a DSN or a schema. No runtime `CREATE SCHEMA`, no
  runtime `SET search_path`, no `DROP SCHEMA` outside test cleanup.
- [x] **RES-D54281's load-bearing claim re-verified: `pgstore` needs zero
  changes.** Schema is purely the connection's `search_path`; production code
  never issues `SET search_path` and reads the schema back via
  `current_schema()`.
- [x] Connection ceiling (RES-S8CH9C, reused by RES-D54281): `pgstore.Open`
  builds a pool with **no `MaxConns`** (defaults `max(4, numCPU)`) plus a
  dedicated non-pooled `LISTEN` connection plus the sweep — ~17 connections per
  open store. TKT-9TOEBH removed the per-tenant *channel*, not the per-tenant
  *pool*.
- [x] `internal/cache.LRU` exists but has **no eviction callback**, so it cannot
  close an evicted store. The registry needs its own recency handling.
- [x] `backendtest_postgres.go:88-116` already creates N isolated schemas on one
  database and hands back a DSN pinned via `dsnWithSearchPath` — which
  re-serializes the parsed config rather than appending `?search_path=`, because
  pgx accepts both URL and key/value DSNs and query-string surgery corrupts the
  latter. Reuse this construction.

## Approach

### Step 1 — `internal/tenant`: the value and the lookup

`Tenant{OrgID, Schema, DSN}`, and `Resolver` with one method
`Resolve(orgID) (Tenant, error)`. A sentinel `ErrUnknownTenant` so callers can
distinguish "not a tenant" from "lookup broke" — both deny, but they are
different operational events.

### Step 2 — the file-backed resolver

`tenants.yaml`: a base DSN plus a list of `{org_id, schema}`. Validation at
load, not at first use: non-empty org, schema matching
`^[a-z][a-z0-9_]{0,30}$`, no duplicate org, **and no duplicate schema** — two
orgs sharing a schema is a cross-tenant leak spelled as a config typo.

Per-tenant DSNs are derived by re-serializing the parsed base DSN with
`search_path=<schema>,public`, the same mechanism as the test harness (`public`
stays on the path for `pg_trgm` operators). An explicit per-tenant DSN override
is allowed, which is what keeps cluster-sharding a config change.

### Step 3 — `Registry`: org -> `*Services`

Holds one `*appbuild.SharedBase` and a `Resolver`. `Acquire(ctx, orgID)`
resolves, then returns a cached `*Services` or assembles one by opening the
backend against the tenant's DSN. Bounded by an operator-set `MaxResident`;
exceeding it evicts the least-recently-used tenant and closes its store.

The awkward part is that eviction must not close a `*Services` a request is
still using. Handle it with explicit reference counting and a release call,
rather than hoping request lifetimes are shorter than eviction pressure — a
use-after-close here is a crash at best.

### Step 4 — the isolation test

On the postgres build, against a live database: create two schemas, build one
shared base, acquire two tenants, write an entity as A, assert it is invisible
as B, and assert the reverse. Then assert the fail-closed cases return an error
and a nil `*Services`.

### Step 5 — arch-lint

Add a `tenant` component. It depends on `appbuild` (it assembles Services),
which means nothing may depend on `tenant` except a future host binary.

## Risk Assessment

- **Use-after-close on eviction** — the sharpest risk. An evicted store closed
  while a request holds it is a panic. Mitigated by reference counting and by
  never evicting an in-use entry.
- **The postgres path may be unverifiable locally.** The isolation test is the
  centrepiece and needs a live PostgreSQL. If none is available, say so plainly
  rather than claiming the path is verified.
- **Cross-tenant mutation through the shared base.** `meta` and `aclPolicy` are
  pointers handed to every tenant. Already pinned by
  `TestSharedBase_AssemblyDoesNotMutateSharedValues`; this work makes a
  violation a cross-tenant defect rather than a curiosity.
- **Scope creep into provisioning.** Deliberately resisted: the registry never
  runs `CREATE SCHEMA`. An unknown org is a denial, and provisioning is
  TKT-TNPRV8.
- **A partially-mounted routing spine could look like isolation without being
  it.** This ticket does not mount the registry in the HTTP path, so nothing in
  the shipping server changes behaviour. That is the honest state and the PR
  must say so rather than implying tenant isolation is live.
