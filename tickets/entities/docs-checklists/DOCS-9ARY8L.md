---
id: DOCS-9ARY8L
type: docs-checklist
title: 'Docs: icon: on navigation, kanban columns and swimlanes'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc/comments on new exported symbols
- [x] Non-obvious decisions explained with WHY, not WHAT

`utils/icons.ts` documents why resolution is a static allowlist and why
`resolveIcon` needs an own-property check (a bare index finds inherited
`Object.prototype` members and would crash the render).

`ValidIconNames` documents that it mirrors the SPA registry and which test pins
it. `NavigationEntry.Icon` explains what the derived default is and why an
override exists. The `.nav-icon` CSS carries a warning that `width` and
`flex-basis` both reproduce the 24x18 stretch bug — the mechanism that caught me
twice.

## Project Documentation

- [x] ~~frontend/CLAUDE.md~~ (N/A: no new frontend convention. The icon
registry is a single documented module, not a pattern other code must follow.)
- [x] ~~CLAUDE.md (root)~~ (N/A)

## External / User-Facing Documentation

- [x] `docs/data-entry.md` — **required**, two new public config keys.

Authored in `docs-project/entities/guides/GUIDE-data-entry.md` (the generated
file is not the source — a lesson from #1282's CI failure).

Two sections: "Column and swimlane icons" under Kanbans, and "Item icons" under
Navigation. Both explain that `icon:` takes a NAME not a glyph, that an emoji
left in a label renders verbatim, and that a group cannot take one.

**The valid-name list is deliberately NOT restated in prose.** It was, twice,
and the rename in RR-OX9WFS made both copies wrong within the hour. The startup
error interpolates the live allowlist, so it is the authoritative reference; the
docs now say so and explain why the omission is intentional.

## Verification

- [x] Documentation matches the implemented behaviour
- [x] Examples in docs actually work

The worked YAML examples are the ones now in the prototype project, so they are
exercised by the running demo rather than only existing in prose. `just
docs-check` (the CI gate) and `markdownlint-cli2` both pass.
