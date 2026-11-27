---
id: TKT-SQ8O0EQ
type: ticket
title: Body task counts as a roll-up (rollup body_tasks)
kind: enhancement
priority: medium
effort: m
started: "2026-11-24"
completed: "2026-11-24"
status: done
---

## Description

Add a roll-up kind that counts markdown task lines (`- [ ]`, `- [x]`) in an
entity's own body: `rollup: { body_tasks: done|open|total }` on an integer
property. It is computed on read, never stored, and usable as a data-entry list
column.

## Acceptance criteria

- Schema declares `rollup: { body_tasks: done }` (or `open`, `total`); the loader rejects other options with it and any type but integer.
- Task lines in fenced code blocks are ignored; `- [X]`, `* [ ]`, `+ [ ]`, numbered and nested items count.
- The value is shown only when the reader can see the body: an unreadable row or an inaccessible body has no value, and a `visible:` grant on the property hides it.
- A list page costs a constant number of store calls and reads only the served rows (budget test).
- Filter and sort on a body-task roll-up are refused, since they would read every body of the type.
- Documented in docs-project with the roll-up options.
