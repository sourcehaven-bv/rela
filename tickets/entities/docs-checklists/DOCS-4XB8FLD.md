---
id: DOCS-4XB8FLD
type: docs-checklist
title: 'Docs: TKT-KW8T8Y5'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why order needs a strict decoder (yaml.v3 truncates fractions), why the phone check reads matchMedia at once, why style and order need available_on
- [x] Function/type docs if public API: godoc on `ActionStyle`, `ActionOrder`, `Builtins.HiddenOnPhone`, `SidebarResponse.PhoneHiddenBuiltins`; TSDoc on `offeredDetailActions` and `usePhoneScreen`

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: extends the existing `actions:` and `builtins:` patterns)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## User-facing Documentation

- [x] `GUIDE-data-entry.md`: `style` and `order` rows in the action fields table and the section "Order and style on the detail page"; `shortcuts` row, `hidden_on_phone` and `phone_hidden_builtins` in the built-in rows section; regenerated docs/data-entry.md
- [x] ~~Metamodel guide~~ (N/A: no schema change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: docs/data-entry/api-reference.md does not document `/_sidebar` or `/_config` action fields)
