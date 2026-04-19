---
id: TKT-MBX8V
type: ticket
title: Fix misfiled entity files in docs-project/entities/
kind: chore
priority: medium
effort: xs
status: done
---

In this repo's `docs-project/` rela instance, entity files are split across
singular- and plural-named folders (e.g. `guide/` vs `guides/`, `feature/` vs
`features/`, plus singular-only `tutorial/`, `scenario/`, `concept/`).

The fsstore convention is `entities/<plural>/<id>.md` (default plural: type +
"s"), so files in the singular folders are parsed with the wrong entity type
(e.g. `guide/` is read as type `guid`) and are effectively orphaned from rela's
view.

No filename overlaps exist between the singular/plural pairs — the folders merge
cleanly.

**Fix:** move every file from the singular folders into their plural
counterparts (creating plural folders where missing), then remove the now-empty
singular folders.

**Scope:** `docs-project/entities/` only. The `tickets/` rela instance is
already correct.

**Affected folders:**

| From | To | Files |
|------|----|----|
| `guide/` | `guides/` | 8 |
| `feature/` | `features/` | 14 |
| `tutorial/` | `tutorials/` (new) | 2 |
| `scenario/` | `scenarios/` (new) | 3 |
| `concept/` | `concepts/` (new) | 7 |
