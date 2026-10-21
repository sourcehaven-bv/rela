---
id: TKT-3IBLIP
type: ticket
title: Create a linked entity from a markdown task item
kind: enhancement
priority: medium
effort: l
started: "2026-10-18"
completed: "2026-10-21"
status: done
---

## Description

In the Milkdown editor, a GFM task item (`- [ ] Write the migration`) can be
turned into a new, linked entity, similar to GitHub's "Convert to issue". The
operator declares the options. A source type can have zero, one or many. With
zero options the editor shows no affordance.

After the conversion, the task item's text is replaced by an entity reference
(`` - [ ] `TKT-XXXX` ``), and a relation links the source entity and the new
entity.

## Design

- A named, server-side recipe. The client sends only the recipe name, the
source entity and the item text. It cannot send its own mapping, as with
`conversions: `.
- Config key: `task_item_conversions: ` in `schema.yaml `. Each recipe declares: the source type (and optionally the property), the
target type, the relation and its direction, a field mapping using
`{{item.text}} ` and `{{source.<property>}} `, and the target face when the
target type has faces.
- The server creates the entity and the relation in one write. The editor
then replaces the item text with the reference, and form autosave stores the
body.
- The server offers only the recipes the principal may use: create on the
target type, the relation, and edit on the source.
- If the mapping leaves required target fields empty, a prefilled create form
opens.

## Decisions

- The key is `task_item_conversions: `, so it is not confused with type
`conversions: `.
- `{{item.text}} ` is the item's own line only. Nested lines and sub-items are
not carried over.
- `item: link | remove ` (default `link `) sets what happens to the task item
after the conversion. `link ` replaces its text with the reference. `remove `
deletes the item; its nested sub-items move up into its place, so no content is
lost.
- A faced target type requires the recipe to name the face to create.
- Create forms do not offer the action, because the source entity does not
exist yet.

## Out of scope

- Ticking the checkbox when the target entity is done. This could be a later
automation.
- Offering "save first, then convert" in create forms.
