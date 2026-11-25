# To-do app

A personal task manager built from rela's existing
features only. The seed data is dated around 2026-11-23.

## Run it

From the repository root:

```sh
go run ./cmd/rela-server -project examples/todo-app -port 8080
# or: just dev examples/todo-app
# or open the folder in rela-desktop
```

Check the project:

```sh
go run ./cmd/rela --project examples/todo-app analyze all
```

The four orphan findings are the Inbox to-dos, which belong to nothing.

## Model

- `area`, `project` (in an area, with a deadline), `heading` (a project's
  sections, in the project's order), `todo`, `tag`.
- A to-do has `status` (open, completed, canceled, a state machine), `when`
  (anytime, someday, scheduled), `start_date` (required when scheduled),
  `part_of_day` (day, evening), `deadline`, `repeat` and `completed_on`. Its
  notes and checklist are the markdown body (`- [ ]`).
- Today is not stored. A scheduled to-do is in Today from its start date on,
  so nothing has to move it at midnight.
- `flows.yaml` stamps `completed_on` when a to-do or project is completed or
  canceled, and clears it when one is reopened. The clearing uses the trigger
  `unset` output, which needs a rela build with TKT-61Y46A0.
- Completing a to-do with `repeat` (`1d`, `2w`, `1m`, `1y`) creates the next
  one, scheduled that interval after its start date, or after today if the
  start date has passed. A small Lua `script` output in `flows.yaml` does it.

## Navigation

| Concept  | rela                                                                      |
| -------- | ------------------------------------------------------------------------- |
| Inbox    | list `inbox`: open, no when, no start date, no project, no area           |
| Today    | list `today`: scheduled with start date reached, or deadline reached; computed groups Today, This Evening, Deadline reached |
| Upcoming | page `upcoming`: list `upcoming` grouped by start date, and calendar `upcoming_calendar` as tabs |
| Anytime  | list `anytime`, grouped by project                                        |
| Someday  | list `someday`                                                            |
| Logbook  | list `logbook`, newest first                                              |
| Areas    | one sidebar group per area with its active projects; area and project pages |

List shortcuts: select rows, then `c` complete, `x` cancel, `t` Today,
`v` This Evening, `a` Anytime, `s` Someday. This Evening runs
`actions/schedule.lua`, which schedules the to-do for today's date in UTC.

Moving rows: Today, Anytime and Someday each declare an `assign:`, the values
a to-do gets when it enters the list. Today's assign schedules it with
`start_date: today()`. Adding a row there, dragging rows onto the list's
sidebar entry, and the `t`/`a`/`s` keys all apply it. Dragging a to-do onto a
project or area entry in the sidebar puts it in that project or area (the
`assign:` on the `project` and `area` pages).
