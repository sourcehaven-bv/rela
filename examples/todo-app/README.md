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
  (today, evening, anytime, someday), `start_date`, `deadline` and
  `completed_on`. Its notes and checklist are the markdown body (`- [ ]`).
- `flows.yaml` stamps `completed_on` when a to-do or project is completed or
  canceled.

## Navigation

| Concept  | rela                                                                      |
| -------- | ------------------------------------------------------------------------- |
| Inbox    | list `inbox`: open, no when, no start date, no project, no area           |
| Today    | list `today`: when is today or evening, or start date or deadline reached; grouped by `when` |
| Upcoming | page `upcoming`: list `upcoming` grouped by start date, and calendar `upcoming_calendar` as tabs |
| Anytime  | list `anytime`, grouped by project                                        |
| Someday  | list `someday`                                                            |
| Logbook  | list `logbook`, newest first                                              |
| Areas    | one sidebar group per area with its active projects; area and project pages |

List shortcuts: select rows, then `c` complete, `x` cancel, `t` Today,
`v` This Evening, `a` Anytime, `s` Someday.

Moving rows: Today, Anytime and Someday each declare an `assign:`, the values
a to-do gets when it enters the list. Adding a row there, dragging rows onto
the list's sidebar entry, and the `t`/`a`/`s` keys all apply it. Dragging a
to-do onto a project or area entry in the sidebar puts it in that project or
area (the `assign:` on the `project` and `area` pages).

Typed dates: the `start_date` and `deadline` fields take phrases as well as the
picker, such as `tomorrow`, `friday`, `next week`, `in 3 days` or `5 march`
(`morgen`, `vrijdag`, `volgende week`, `over 3 dagen`, `5 maart` in Dutch). The
field shows the date before it applies; Enter or leaving the box applies it.
