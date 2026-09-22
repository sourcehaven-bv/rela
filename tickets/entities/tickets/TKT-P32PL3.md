---
id: TKT-P32PL3
type: ticket
title: Surface list/view/dashboard command contexts in the SPA (only entity context renders today)
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

The backend serves four command contexts (entity, list, view, global/dashboard)
but the Vue SPA only ever fetches `pageType: 'entity'` from `EntityDetail.vue`.
Commands declared `context: list` / `view` / `global` are reachable over HTTP
yet have no UI affordance — they silently never appear. Add the missing surfaces
(list toolbar, view page, dashboard) so advertised contexts are actually
reachable.

## Summary

`resolveCommands` (`internal/dataentry/commands.go:47`) and `GET
/api/v1/_commands` (`internal/dataentry/api_v1.go:110`, handler at `:3296`)
fully support four contexts. Backend tests cover global/list/view
(`internal/dataentry/commands_test.go:689,719,739`).

The frontend calls `getCommands` in **exactly one place**, hardcoded to
`entity`:

```js
// frontend/src/components/entity/EntityDetail.vue:321-324
commands.value = await getCommands(
  { pageType: 'entity', entityType: props.entityType },
  localAbort.signal
)
```

It never passes `qualifier`, so even `view` is unreachable. `ListView.vue` is an
11-line shell over `EntityList.vue`, which renders only the list's own row
`actions:` (via `useListActions`) — and only in the selection header row
(`EntityList.vue:823-845`), i.e. bulk/selection-scoped, never a persistent
toolbar.

So `context: global` / `list` / `view` is silently a no-op in the UI.

## Vocabulary note (important, not in the original report)

`context:` (yaml, authored per command) is one of `entity | list | view |
global` — `internal/dataentryconfig/validate.go:95`. The `page_type` query param
the SPA sends uses `dashboard` where the config says `global`. There is no
`page_type=global`. List-toolbar work must send `page_type=list`.

## Impact

Motivating case: a "Generate Statement-of-Applicability PDF" button on
`/list/all_maatregels`. The command works end-to-end via `POST
/api/command/<id>` and is correctly returned by the API, but no button renders
anywhere. Fallback was to redeclare it `context: entity` on an unrelated detail
page.

The separate `actions:` mechanism is the wrong shape for this: it requires row
selection (`if selectedIds.size === 0 return`) and runs once per selected
entity, whereas the need is one invocation covering the whole list/filter.

## Expected

- `list` → toolbar button on the list view, acting on the whole list / current
filter, **not** requiring row selection (distinct from row `actions:`)
- `view` → button on the view page
- `global` → button on the dashboard (and/or sidebar entry)

At minimum, document that only `entity` renders today, so the other contexts
aren't silently a no-op.

## Known blocker

`CommandModal.vue` hardcodes `entityId`. A list/dashboard invocation has no
entity, so the modal needs to accept a context-shaped payload before the toolbar
button can work. `handleCommandExec` already builds the right stdin per context
(`commands.go:301-340`): `list` needs `list_id`, `view` needs
`view_id`+`entity_id`, `global` needs nothing.

## Natural implementation seam

`EntityList.vue:662-673` — the `<header class="list-header">` block that already
holds `BackButton`, `<h1>`, and the `+ New` link. A `v-for="cmd in commands"`
row there structurally mirrors `EntityDetail.vue:716-724`.

## Adjacent defect found while investigating

When `available_on` is present, `cmd.Context` is **not consulted at all**
(`matchesPage`, `commands.go:77-108`). A command with `context: entity` and
`available_on: {lists: [tickets]}` matches `page_type=list`, then
`handleCommandExec` switches on `cmd.Context`, builds an entity payload, and
404s on the missing `entity_id`. `validateCommands` does not catch the
disagreement, and does not validate `dashboard: true` against context. Worth
fixing alongside.

## Repro

1. Declare a `commands:` entry with `context: global`, `available_on: { dashboard: true }`.
2. `GET /api/v1/_commands?page_type=dashboard` → returns the command. ✅
3. Open the dashboard (or any list) in the SPA → no button appears. ❌

## Version

Reproduced on `f25fa238` and current `develop` — both affected.
