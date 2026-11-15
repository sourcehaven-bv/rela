---
id: TKT-WJP6PBM
type: ticket
title: 'Settings: search picker for relation defaults'
kind: enhancement
priority: low
effort: s
status: backlog
---

## Description

Follow-up of TKT-F2EJQY8. `GET /api/v1/_settings` lists every readable entity of
every relation type's target type, as the choices for a relation default
(`settingsRelationTargets` in `internal/dataentry/settings_handlers.go`). Since
TKT-F2EJQY8 the read is gated and content-free, but it still holds every row of
each target type, on every settings page load.

Steps:

- Replace the relation-default `<select>` in `frontend/src/views/SettingsView.vue`
with a search picker backed by `/api/v1/_search?q=type:<target> <text>&limit=20`
(or the list endpoint with `?q=`), showing the saved default's title from a
single entity read.
- Drop `targets` from `APIRelationDef` and the `SettingsRelationTarget` type.
- Remove the whole-type read from the settings handler.
- Follow `frontend/CLAUDE.md` (component tests, i18n strings).
