---
id: TKT-TX961Z
type: ticket
title: Basecamp connector syncs all to-dos and models projects
kind: enhancement
priority: medium
effort: m
started: "2026-10-09"
completed: "2026-10-09"
status: done
---

## Description

The Basecamp example syncs one to-do list, set by the secret
`basecamp_todolist_id`. Sync every to-do the account can see instead, and model
Basecamp projects and to-do lists as rela entities.

- Pull projects and their to-do lists into `bc-project` and `bc-todolist` entities, linked by relations.
- Pull every to-do into `todo`, linked to its to-do list.
- A todo created in rela is pushed to the to-do list it is linked to.
- Bodies are converted with `rela.md.from_html` / `rela.md.to_html`.
