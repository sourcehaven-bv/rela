---
id: DOCS-EHS7A6
type: docs-checklist
title: 'Docs: incoming relation order reorder'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (authorizeIncomingSiblings and why it runs before the Tx, orderSiblingFilter tail rule, orderRef address parsing, the caller-side edge gate before newRelationOrdering, per-source order visibility)
- [x] Function/type docs if public API (entity.OrderPosition incl. Incoming and keyed Among, v1.RelationOrder Direction and Addresses, the SPA RelationOrder type, moveRelation direction, orderAddress and orderMoveArgs)

## Project Documentation

- [x] ~~README updated~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated~~ (N/A: no new cross-cutting pattern; the sibling-authorization rule is documented in docs/data-entry/api-reference.md and the godoc on authorizeIncomingSiblings)
- [x] ~~Help text accurate~~ (N/A: no CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: no changelog file in this repo)
- [x] API docs updated (docs/data-entry/api-reference.md: incoming `direction` on "Moving an edge (`position`)", source addresses, the sibling-authorization rule, the answers table, and `direction`/`addresses` under "Reading the order"; docs/data-entry.md "Rows in relation order" and docs/metamodel.md "Ordered Relations" regenerated from GUIDE-data-entry and GUIDE-metamodel)
