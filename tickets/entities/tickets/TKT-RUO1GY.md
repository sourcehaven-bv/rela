---
id: TKT-RUO1GY
type: ticket
title: 'Feed subscriptions: poll RSS/Atom/JSON Feed into entities'
kind: enhancement
priority: medium
effort: l
started: "2026-10-14"
completed: "2026-10-14"
status: done
---

## Description

Operators can declare feeds (RSS 2.0, Atom, JSON Feed) that rela polls on a
schedule and imports as entities.

The Lua half shipped separately: `rela.xml.decode` and `rela.feed.parse` with
hardened XML decoding (TKT-6KN0AC, #1879). The mapping core moved into
`internal/ingest` (TKT-RQ6BSA, #1883). This ticket adds the declarative
subscriptions on top of both.

## Design direction (from discussion)

- **Own config file**: `subscriptions.yaml` at the project root. Polling is
periodic work, closer to `schedules.yaml` than to `data-entry.yaml`. Both `rela
scheduler` and `rela-server` read it. The key `feeds:` is already taken by
calendar/feed export.
- **Mapping reuses the webhook vocabulary** (`find:` + `match:`,
`create_if_missing:`, `then:` with `set` / `append_section`), with `{{item.*}}`
instead of `{{body.*}}`. Identity is a `computed:` + `unique:` property on the
entity, as with webhooks.
- **Execution**: the scheduler enqueues one job per subscription on
`jobs.Queue`. `ETag` / `Last-Modified` live in `state.KV`, so a 304 writes
nothing. Writes go through `entitymanager` as a `system:subscription:<name>`
principal.
- **Auth** for private feeds references `.rela/secrets.yaml`; the secret
never appears in config.
- **Untrusted item content**: item HTML is converted to markdown with
`htmlmd.FromHTML`, which sanitizes it with bluemonday first. Raw feed HTML never
reaches an entity (review requirement from #1879).
- **Shared validation**: the metamodel-free and metamodel checks of a mapping
move into `ingestspec` so webhooks and subscriptions report the same errors
(RR-HEF5VV).

## Out of scope

- iCalendar import and generic JSON API polling.
- Managing subscriptions from the UI (they stay operator config).
