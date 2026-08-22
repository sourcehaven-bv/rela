---
id: FEAT-5K7FOV
type: feature
title: Calendar views for data-entry
summary: 'Configurable day/week/month/year calendar views in the data-entry SPA declared under calendars: in data-entry.yaml the same way kanbans: are — with drag-to-reschedule writing back to the entity date property. Sibling of FEAT-OT4361 (read-only feed export) not a duplicate of it.'
description: 'An in-app interactive calendar view type for the data-entry web app configured declaratively under calendars: in data-entry.yaml the same way kanbans: are. Distinct from FEAT-OT4361 which is read-only feed export (ICS/CalDAV) for foreign clients.'
priority: medium
status: proposed
---

## Summary

An in-app, interactive **calendar view type** for the data-entry web app — day,
week, month and year — configured declaratively under `calendars:` in
`data-entry.yaml`, exactly the way `kanbans:` are configured today.

## Motivation

rela can already *export* time-bearing entities to foreign calendar clients
(FEAT-OT4361: read-only ICS/JSON feeds, and the CalDAV work behind RES-1Y2EB5).
What it cannot do is **show you a calendar inside rela itself**.

A user whose graph contains tasks with due dates, meetings with dates, or
milestones has to subscribe an external client to see them laid out in time. For
the same reason kanban exists — status is an enum, so show it as columns — dates
deserve a native spatial rendering.

## Relationship to FEAT-OT4361 (feed export)

These are siblings, not duplicates:

| | FEAT-OT4361 (feeds) | This feature (calendars) |
|---|---|---|
| Direction | Read-only export | Interactive, reads and writes |
| Consumer | Foreign clients (Apple Calendar, Thunderbird) | The rela SPA |
| Model | `calfeed.Event` — RFC 5545 vocabulary, deliberately lossy | Entities, with identity, type, ACL |
| Interaction | None | Click to edit, drag to reschedule |

The overlap is exactly one thing: the **entity-to-event source mapping**
(`entity_type`, `where`, `date`, `end_date`, `summary`, `description`).

Deliberate decision: `calendars:` declares its **own** source fields rather than
sharing a Go type with `feeds:`. Operators who want one declaration to serve
both use a YAML anchor. The advantage of independence is that an incompatible
change to either block stays cheap — users copy a bit of config instead of the
project being locked into a shared schema.

## Scope

- A `calendars:` block in `data-entry.yaml`, mirroring `kanbans:` structurally
- Period views, navigation between periods, and "today"
- Events sourced from one or more entity types via a source mapping
- Click an event to open the entity or a configured `edit_form`
- Drag an event to reschedule, patching the date property (ACL-gated,
mirroring kanban's drag-to-change-status)
- Sidebar/navigation integration and `permission:` gating, like other views

## Explicitly not in this feature

- Publishing a calendar view as an ICS feed (natural follow-up; the config
design should leave room for it)
- CalDAV two-way sync (RES-1Y2EB5, separate arc)
- Recurrence *expansion* in the view — `rrule:` is a feed export concern
