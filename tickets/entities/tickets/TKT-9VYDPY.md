---
id: TKT-9VYDPY
type: ticket
title: 'Postgres-backed config store: schema and app config as database-native data'
kind: enhancement
priority: medium
effort: xl
status: backlog
---

## Problem

Store projected config in PostgreSQL rather than YAML files, so config is
database-native. This is the piece that makes **tenants building their own apps
from the web** possible: a tenant's schema and data-entry config live in the
database alongside their data, not in an operator-authored repo file.

## Why this is simpler than the YAML store

A Postgres-backed config store **has no comments to preserve**. The
AST/diff-apply machinery of TKT-K58O37 is unnecessary here: the graph *is* the
source of truth, and YAML becomes an export format rather than an authored file
to protect.

That inverts the difficulty. The hard problems move to:

- **one-time import** of existing YAML config into the store
- **export fidelity** — producing readable YAML from the graph
- **coexistence** — operator-authored files and tenant-authored config in one deployment

## Relationship to the YAML store

These are **parallel store implementations, not alternatives**. Both are
plausibly wanted: YAML for repo-authored projects (the current model,
git-friendly, reviewable in PRs), Postgres for tenants authoring from the web.
They share the projection model (TKT-2WVTRA) and differ only in persistence.

## Note on the metamodel-from-disk rule

`CLAUDE.md` currently states the metamodel is **always read from disk, even in
the postgres build** — `schema.yaml` and `templates/` stay on the filesystem,
and a postgres deployment still needs a `--project` dir. This ticket directly
revisits that constraint, so it needs an explicit decision entity rather than a
quiet change. The `state.KV` precedent (TKT-VC27L3) is the closest analogue:
runtime-written state moved into `state_kv` on the postgres build precisely
because node-local state breaks multi-process deployments — the same argument
applies to tenant-authored config.

## Open questions (resolve when work starts)

- **Does this replace the disk metamodel for postgres deployments, or coexist with it?** Likely coexist: operator-authored base config on disk, tenant-authored overlays in the database. If so, how do they compose, and which wins on conflict?
- **Schema-per-tenant or a config table keyed by tenant?** The existing postgres backend already scopes by schema, which would scope config for free.
- **How does a config change propagate to running processes?** The `LISTEN/NOTIFY` change feed (TKT-WZYWM9) exists for entity writes; config changes need reload, which is a heavier operation than an event.
- **Where does validation live?** A tenant writing an invalid schema must not break their own deployment — nor be able to affect another tenant's.
- **Does the projected config graph live in the SAME store as tenant data** (so `analyze` sees both) or a separate one? Separate is cleaner; same makes cross-cutting queries possible.

## Context

Findings `.ignored/schemaspike/FINDINGS.md` §8. Depends on the projector;
independent of the YAML store.
