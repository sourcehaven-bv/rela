---
id: FEAT-UP14BT
type: feature
title: 'Self-contained SQLite: operator config in the database'
summary: Ship one .db file that is a fully working rela app — schema, config, scripts, templates and data — given only the rela binary
description: 'Config-in-the-DB as a MODE, not a migration: resolution is disk-first with DB fallback, so a normal project keeps schema.yaml on disk and behaves exactly as today. `rela db dump`/`db load` round-trips config in and out. Delivered per-seam (config.Loader, metamodel.Loader, templating.Templater, a script source, a policy source, state.KV) rather than by faking a filesystem over SQLite.'
priority: medium
status: planned
---

## Goal

One `.db` file that, given only the `rela` binary, is a fully working app —
schema, data-entry config, ACL, scripts, templates, custom assets, and the
entity/relation data. Editing config happens via a dump & load round-trip.

## Why this is tractable

Four of the five seams needed already exist and were built anticipating this:

| Seam | Location | Backends today |
|---|---|---|
| `config.Loader` | `internal/config/config.go:20` | `FSLoader` |
| `metamodel.Loader` | `internal/metamodel/loader_service.go:19` | `FSLoader` |
| `templating.Templater` | `internal/templating/storetemplate.go:42` | `FSTemplater` |
| `state.KV` | `internal/state/state.go:23` | `FSKV` + `pgstore.StateKV` |

`config.Loader`'s package doc already says *"remote or **embedded** deployments
plug in by implementing Loader."* `storetemplate.go` is named for a store-backed
Templater that was planned and never built. `state.KV` is the proof the pattern
works end to end, including a conformance suite.

The SPA is already `go:embed`-ed and attachments are already blobs in the same
SQLite database, so neither needs work.

## Bootstrap order

Separating the SQLite *connection* from the *store* removes the apparent
ordering problem:

```
1. acquire sidecar process lock       (already in OpenContext)
2. sql.Open + PRAGMA verify + schema  (already in Store.init)
   ─── connection now usable ───
3. load metamodel                     ← may read from the connection
4. load acl.yaml, data-entry.yaml, …  ← may read from the connection
5. sqlitestore.New(conn, …) → store.Store
6. assemble(base, store, …)           (unchanged)
```

This is the shape `pgstore` already has: `pgstore.New(db DBTX)` takes an
injected pool and appbuild owns/closes it. `SharedBase` survives — it is
documented as holding nothing derived from a store, and a `*sql.DB` is not a
store.

## Not in scope

`.rela/secrets.yaml` stays on disk: it has OS-keychain and systemd-credentials
integration plus permission warnings, and a credential inside a shippable
artifact is exactly the leak the "configuration is not a secret; the data is"
rule prevents. `tenants.yaml` stays on disk for the bootstrap circularity its
own doc comment describes.

## Supersedes

The scope note at `internal/appbuild/appbuild_sqlite.go:19-24` ("it does not
swallow operator-authored config"). Reversed deliberately, and narrowly — disk
stays the default and stays first in resolution order.
