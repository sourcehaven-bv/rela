---
id: TKT-G8Y21J
type: ticket
title: Document and e2e-test Create menu linking on entity pages
kind: docs
priority: medium
effort: s
started: "2026-10-06"
completed: "2026-10-06"
status: done
---

## Description

BUG-PFLS22 made the space Create menu link a new item to the entity page it was
created from. Two parts of that fix were left out:

- `docs/data-entry.md` § Entity pages only says that New and a section's Add
link to the page entity. It does not mention the Create menu, the timeline tab
(`scope: root`), or what happens when the page offers several relations for the
created type.
- No e2e test drives the Create menu on an entity page. Only unit tests cover
it.

The YAML example in that section also uses non-English names. They are replaced
with English ones.

## Acceptance criteria

- § Entity pages states that the Create menu links like New, including on a
timeline tab, and gives the rules: the open tab's relation wins; otherwise the
page's single relation for the type is used; with two different relations, or
none, no link is made.
- The section's example uses English names.
- An e2e test opens an entity page in a space, creates an item through the
Create menu and checks the relation for a list tab and a timeline tab, and
checks that no link is made when two relations fit.
