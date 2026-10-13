---
id: TKT-JT101B
type: ticket
title: 'Section hide_empty: leave an empty section out of the view'
kind: enhancement
priority: medium
effort: s
started: "2026-10-12"
completed: "2026-10-12"
status: done
---

## Description

A view section always renders, and shows `empty_message` (or "No items") when
its collection is empty. Some sections only matter on some entities, such as a
"corrective action for" section on a task. On other entities the empty section
is noise.

Add an opt-in section key `hide_empty: true`. When the section's collection has
no rows, the server leaves the whole section, heading included, out of the
`_views` response.

Refused at config load:

- with `empty_message`, because a hidden section shows no message;
- with `create:`, because the button would vanish when the section is empty;
- with `source: entry`, which has no collection that can be empty;
- in a form's `side_panel`, whose sections carry Add and Link buttons.

A section whose collection is incomplete (`mentioned_by` limit) stays.

The setting travels on `SectionData` as an unexported field, so the struct
stays within the god-object lint's limit of 20 exported fields.

## Acceptance criteria

- [x] An empty section with `hide_empty: true` is absent from the `_views` response.
- [x] An empty section without it still renders.
- [x] The create affordance on a later section is unaffected.
- [x] Each refused combination fails config load with a message naming the key.
- [x] docs/data-entry.md documents the key and its refusals.
