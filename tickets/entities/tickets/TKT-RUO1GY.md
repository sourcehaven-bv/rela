---
id: TKT-RUO1GY
type: ticket
title: 'Feed subscriptions: poll RSS/Atom/JSON Feed into entities, plus rela.xml / rela.feed for Lua'
kind: enhancement
priority: medium
effort: l
status: backlog
---

## Description

Operators can declare feeds (RSS 2.0, Atom, JSON Feed) that rela polls on a
schedule and imports as entities. Lua scripts get the same parsing primitives so
an odd feed can be handled in a script.

## Design direction (from discussion)

- **Own config file** (`subscriptions.yaml` at the project root, name to be
settled in planning). Polling is periodic work, closer to `schedules.yaml` than
to `data-entry.yaml`. Both `rela scheduler` and `rela-server` read it. The key
`feeds:` is already taken by calendar/feed export.
- **Mapping reuses the webhook vocabulary** (`find:` + `match:`,
`create_if_missing:`, `then:` with `set` / `append_section`), with `{{item.*}}`
instead of `{{body.*}}`. The mapping core moves out of `internal/dataentry` so
both callers share it. Identity is a `computed:` + `unique:` property on the
entity, as with webhooks.
- **Execution**: the scheduler enqueues one job per subscription on
`jobs.Queue`. `ETag` / `Last-Modified` live in `state.KV`, so a 304 writes
nothing. Writes go through `entitymanager` as a `system:subscription:<name>`
principal.
- **Auth** for private feeds references `.rela/secrets.yaml`; the secret
never appears in config.
- **Untrusted item content**: HTML is converted to markdown via `htmlmd`.
- **Lua**: add `rela.xml.decode(string)` (XML to a plain table) and
`rela.feed.parse(string)` (the same parser as the declarative path). `rela.json`
already exists.

## XML safety

Build on `encoding/xml`: it does not resolve external entities (no XXE), does
not process DTDs, and expands only the five predefined entities (no
billion-laughs). Go 1.19+ caps nesting depth. On top of that: input size cap,
our own depth / element / attribute caps, `Strict = true`, a fixed charset
allowlist (UTF-8, ISO-8859-1), and no XPath or DTD surface in Lua. Pin each with
a hostile-input test.

## Out of scope

- iCalendar import and generic JSON API polling.
- Managing subscriptions from the UI (they stay operator config).
