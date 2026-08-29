---
id: DOCS-TNT9RS
type: docs-checklist
title: 'Docs: tenant resolution spine'
status: done
---

## Code Documentation

- [x] Package doc on `internal/tenant` states why the package exists: isolation
  cannot be an ACL rule (union semantics, no deny primitive), so it comes from
  the store handle instead. It also states the fail-closed rule and names the
  existing fail-open traps a reader should not accidentally join.
- [x] `Tenant`, `Resolver`, `MapResolver`, `Registry`, `Lease` and `Opener` all
  carry doc comments explaining the decision behind them, not a restatement of
  the signature.
- [x] The tenant-map storage decision is recorded where the type lives
  (`Config`'s doc in `config.go`), with the recursion argument that drove it —
  the map cannot live inside the thing it shards, so a config file is the
  bootstrap layer that a database-backed map would sit on top of rather than
  replace.
- [x] The connection-ceiling reasoning is recorded on `DefaultMaxResident`,
  including the ~17-connections-per-store measurement from RES-S8CH9C and why
  the default is deliberately small.
- [x] `AppBuildOpener` documents the `appbuild.New` / `WithDatabaseURL`
  asymmetry — the option is consumed by `Discover` while `New` reads the field,
  so passing the option here would compile, do nothing, and put every tenant on
  one DSN. That trap is worth a paragraph because it is silent.
- [x] `dsnForSchema` documents why the DSN is rebuilt by re-serializing the
  parsed config rather than by appending `?search_path=`: pgx accepts two DSN
  dialects and query-string surgery corrupts the key/value one into a DSN that
  still connects, on the wrong schema.
- [x] The `!postgres` stub documents why it errors rather than returning the
  base DSN — an unpinned DSN would put every tenant on one `search_path`.
- [x] Test doc comments state what property each test pins and what a failure
  would mean, rather than describing the mechanics.

## Project Documentation

- [x] TKT-TNT9RS records both open questions RES-D54281 left, with reasoning:
  where the tenant map lives, and how the connection ceiling and resident-set
  bound are honoured.
- [x] PLAN-TNT9RS records the re-verification done against develop rather than
  against ticket statuses, including the `appbuild.New` / `WithDatabaseURL`
  asymmetry that changed the implementation approach.
- [x] Deferred work is filed as TKT-TNPRV8 (provisioning) and TKT-TNERAS
  (erasure), each carrying the design questions RES-D54281 raised for it — the
  trust assumption behind lazy provisioning, and the audit-log residue and
  backup-retention SLA for erasure.
- [x] ~~`docs/postgres-backend.md` updated~~ (N/A: the registry is not mounted
  in any binary, so there is no operator-facing behaviour to document yet.
  `tenants.yaml` becomes operator documentation when a host reads it —
  documenting a config file no binary loads would be documenting a plan.)
- [x] ~~`docs/acl-security.md` updated~~ (N/A: nothing about ACL evaluation
  changed. That document's statement that `org_id` is "recorded, not enforced"
  remains accurate on every shipping path, and would become wrong only when the
  registry is actually mounted.)
- [x] ~~CHANGELOG~~ (N/A: no user-visible behaviour change.)
