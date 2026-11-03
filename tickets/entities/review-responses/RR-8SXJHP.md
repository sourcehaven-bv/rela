---
id: RR-8SXJHP
type: review-response
title: The prefill must seed the type's draft first, or a required boolean shows as empty
finding: '`select(type, values)` runs before the `current` watcher, which seeds a new type''s draft only when it is missing (RlChannelComposer.vue:272-278). If the prefill creates `drafts[type]` from the values alone, the watcher skips seeding. A required boolean then stays undefined, `check()` reports it as required (line 341), and the focus rule picks it as "required and empty". This contradicts the plan''s edge case "the composer seeds it as false". Write `(drafts[type] ??= seeded(def))` before applying values, and add a story case for a type with a required boolean.'
severity: minor
resolution: 'ADDRESSED Moot: the prefill no longer writes the draft, so the existing watcher seeds it and a required boolean starts false. ComposeModeFocus pins this (AC 8).'
status: addressed
---
