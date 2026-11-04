---
id: TKT-6EEXXE
type: ticket
title: 'Finish next actions: every page surface, context-aware sources, per-view and global placement config'
kind: enhancement
priority: medium
effort: l
started: "2026-10-11"
completed: "2026-11-03"
status: done
---

## Description

The next-action layer (FEAT-79DTF9) is only half wired into the UI:

- The page-level card (`banner` / `notice` prominence) renders only on the
dashboard. On every other page a banner-band suggestion is invisible; only the
status-bar chip appears.
- Context-aware sources (`context: <entity_type>`) are parsed, validated and
documented ("scopes the source to the entity being viewed"), but never resolved.
`nextActionCandidates` returns nothing for them, so they never fire.
- The operator cannot say where suggestions appear: there is no per-view or
global switch, and no way to narrow which sources compete on a given page.

## Goal

Suggestions work on every page: list, kanban, calendar, gantt, entity detail,
documents, pages and the dashboard. The operator configures where they appear
globally (per surface kind) and per view (on, off, or a source allowlist).
Context-aware sources resolve against the entity being viewed.

## Notes

- Because suggestions now appear on every page, list pages early in the worlds
manual used up the `finished-draft` candidates before the dashboard was shown.
The prototype places both of its sources on the dashboard only.
- The page `suggestions:` switch made `dataentryconfig.Page` differ from the
sidebar payload. Both now embed `dataentryconfig.PageHeader`, so the header
fields are declared once.
