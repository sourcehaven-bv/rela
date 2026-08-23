---
id: TKT-IG54YO
type: ticket
title: Calendar views (month + week) for data-entry
kind: enhancement
priority: medium
effort: l
status: review
---

## Description

Add a **calendar view type** to the data-entry web app: entities with a date
property laid out in **month** and **week** grids, navigable between periods,
with **drag-to-reschedule** writing the new date back to the entity.

Configured declaratively under a new `calendars:` block in `data-entry.yaml`,
mirroring how `kanbans:` are configured — so an operator who knows kanban
already knows this.

## Decisions settled during ticket intake

These were discussed and resolved with the user before planning:

### 1. `calendars:` is independent of `feeds:` — YAML anchors provide reuse

`feeds:` (FEAT-OT4361) already declares an entity-to-event mapping
(`entity_type`, `where`, `date`, `end_date`, `summary`, `description`) for
ICS/CalDAV export. `calendars:` will declare its **own** source fields, using
the same field names, **without** extracting a shared Go type and **without**
refactoring the shipped feed config.

Rejected alternative: extract a shared `EventSource` type embedded by both.
Rejected because it buys validated consistency at the price of refactoring an
`implemented` config with a live CalDAV consumer.

Operators who want one declaration serving both use a YAML anchor —
`gopkg.in/yaml.v3` resolves anchors and merge keys at parse time, so no Go code
needs to know:

```yaml
_task_events: &task_events
  - entity_type: task
    where: ["status != done"]
    date: due_date
    summary: title

feeds:
  tasks:
    sources: *task_events
calendars:
  schedule:
    sources: *task_events
```

**Rationale (user's):** keeping the structs independent means an incompatible
change to either block stays cheap — users copy a bit of config, rather than the
project being locked into a shared schema.

**Follow-up:** YAML reuse deserves its own guide, since this pattern will come
up in more places than calendars. Filed separately.

### 2. Drag-to-reschedule IS in scope

Mirrors `KanbanView.vue`'s drag-to-change-status: `updateEntity` with a property
patch, gated by the same `canUpdate` ACL check.

### 3. Month + week only; day and year deferred

Day and year views follow in a second ticket. Year view in particular is a
distinct rendering (12 mini-months with density indicators, not event text).

### 4. A non-writable `date:` fails the config load

Validation at load time asserts `date:` / `end_date:` are real properties of the
entity type. A misconfigured calendar fails loudly at startup rather than
silently breaking on the user's first drag — consistent with the project's
"constructors reject nil required fields" rule.

## Proposed configuration shape

Indicative, to be confirmed in planning:

```yaml
calendars:
  schedule:
    title: "Schedule"
    default_view: month          # month | week
    header: |                    # markdown info region, as kanban has
      Drag an event to reschedule it.
    sources:
      - entity_type: task
        where: ["status != done"]
        date: due_date
        end_date: end_date       # optional; must be same kind as date
        summary: title
    edit_form: edit_task
    create_form: create_task
    filters: []
    filter_controls: []
    permission: view_schedule
```

## Acceptance criteria

1. A `calendars:` block in `data-entry.yaml` renders a working calendar view
reachable from the sidebar via a `calendar:` navigation entry.
2. Month view lays out a full month grid; week view lays out seven days.
Both indicate "today" and support next/previous/today navigation.
3. Events come from one or more `sources:`; multiple sources merge into one
calendar (as feeds do).
4. All-day (`date`-typed) and timed (`datetime`-typed) events both render;
a source's `date` and `end_date` must be the same kind.
5. Clicking an event opens the entity, or `edit_form` when configured.
6. Dragging an event to another day patches the date property. When
`end_date` is set, the duration is preserved (both move).
7. A user without update permission on the entity cannot drag it — the same
`canUpdate` gate kanban uses.
8. A calendar whose `date:` names a non-existent or non-writable property
fails config load with a clear error naming the calendar and property.
9. Entities hidden by the read-side ACL never appear as events.
10. `permission:` on a calendar gates it exactly as it gates other views.

## Out of scope

- **Day and year views** — follow-up ticket
- **Publishing a calendar as an ICS feed** — follow-up; config should leave room
- **CalDAV two-way sync** — RES-1Y2EB5, separate arc
- **Recurrence expansion** — `rrule:` stays a feed-export concern; a recurring
entity renders on its base date only
- **Drag-to-resize** (changing duration by dragging an event edge)
- **Overlapping-event layout algorithms** beyond a simple stack in week view

## Prior art in-tree

- `FEAT-006` / `frontend/src/views/KanbanView.vue` (~1043 lines) — the view
pattern to copy: config struct, validation, wire type, sidebar entry, ACL gate,
drag-to-write via `updateEntity`
- `internal/calfeed` — `Event` model (all-day vs `Timed`), already vendor-neutral
- `internal/dataentryconfig` — config parsing and validation
- `datetime` is already a real metamodel property type
(`metamodel.PropertyTypeDatetime`), so timed events are not blocked
