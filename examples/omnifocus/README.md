# OmniFocus-style task manager

This example models OmniFocus in rela. It is a probe: its purpose is to find
what rela lacks for an advanced task manager. The gaps are listed below.

Run it with `rela-server -project examples/omnifocus`.

## What maps

| OmniFocus                         | rela                                                                 |
| --------------------------------- | -------------------------------------------------------------------- |
| Folders, projects, actions, tags  | Entities `folder`, `project`, `action`, `tag`                        |
| Projects and action groups        | Relation `child_of` (action → action or project), ordered            |
| Nested tags                       | Relation `tag_parent`                                                |
| Project status (active, on hold…) | State machine `project_status`                                       |
| Parallel, sequential, single      | Enum `ordering`; the `waiting` roll-up                               |
| Held by a parent or a tag         | The `held`, `held_by_tag` and `deferred_above` roll-ups              |
| Complete with last action         | `scripts/action_status.lua`                                          |
| Repeat (fixed, defer/due again)   | `repeat_rule` (RRULE) and `repeat_method`; `action_status.lua`       |
| Defer, planned and due dates      | Properties; the Forecast list and calendar                           |
| Review                            | `review_every`, `last_reviewed`, computed `next_review`, Review list |
| Inbox                             | List of open actions with no parent and no tag; quick entry          |
| Available, Remaining, Flagged     | Lists with `condition:` that read the roll-ups                       |
| Stalled projects                  | List on the `has_available_action` roll-up                           |
| Checklist in notes                | `checklist_done` and `checklist_total` roll-ups over body tasks      |
| Keyboard actions                  | List actions: complete, drop, flag, defer, mark reviewed             |

## Availability

Availability is computed by roll-ups. No script writes it, so it stays
correct when a row is moved, reordered by drag or changes status.

- `held` walks up `child_of` and `in_folder`. It is true when a group above
  is not open, the project is on hold, dropped or completed, or a folder
  above is dropped.
- `held_by_tag` walks `tagged` and then up `tag_parent`. It is true when a
  tag on the action, or a parent of that tag, is on hold or dropped.
- `deferred_above` is true when a group above or the project has a defer
  date in the future.
- `waiting` is true when, in a sequential parent, an earlier sibling is still
  open. With `climb: true` it also checks every group above.
- `has_available_action` on a project is true when a direct child passes all
  four checks and the rest of the Available list's rules: its own defer date
  is not in the future, and it is not a group with open children. It reads
  `subactions_open` for the last rule. Open work inside a group is one level
  too deep for it, so such a project shows as stalled.

Each roll-up only sees the rows the reader may see. A hidden parent or
sibling does not count.

## Bugs found

1. **`relation_created` and `relation_removed` triggers never fire.** They
   are documented and pass validation, but the entity manager runs no
   automation on relation writes. The example no longer depends on them.
2. **One bad condition silently turns off every condition.** If one list or
   view condition fails to compile, the server starts without an error and
   every list returns every row. `rela validate` reports the error.

## Gaps

Hierarchy:

- `owning:` relations cannot nest, so action groups are plain relations.
- `display: nested` views show two levels, refuse `recursive: true`, and
  cannot be a tab on an entity page. There is no outline editing (indent,
  outdent, move under).
- A view traverse condition cannot test relations at all, so the project
  outline cannot keep subactions off the top level.
- `group_by: relation` needs a relation with one target type. `child_of`
  has two, so the Available and Remaining lists are not grouped by project.

Conditions and properties:

- `related()` cannot compare a boolean property.
- A roll-up condition that uses `now()`, such as `deferred_above`, does not
  lower to SQL. Lists that read it are filtered in Go on every backend.
- A `!=` filter excludes rows where the property is unset, and enum defaults
  are not stored, so `ordering != single_actions` drops projects with no
  ordering. The stalled list uses `~=` in a condition instead.
- `date_add()` in a computed property refuses an integer property as its
  count, so the review interval is an enum.
- Date conditions use UTC, not the user's time zone.
- A `list_id` API request applies `condition:` but not static `filters:`;
  the SPA sends those as query parameters.

Workflow:

- Repeats must be written in Lua. Reopening and completing a repeating action
  again creates a second copy.
- The script sandbox has no `os` date functions; `lib/of/tree.lua` computes
  day numbers by hand.
- No reminders or notifications at a due or defer time.
- Tags cannot have a location or geofence.
- Custom perspectives can only be added by editing config, not at runtime.
- Forecast places each row in one date bucket, so an action with a defer and a
  due date appears once.
- Quick entry needs a list with `assign:`.
- Grouped lists show empty groups for projects whose actions are all hidden.
