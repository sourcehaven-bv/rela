---
id: RR-HUNK
type: review-response
title: Manual-type substring label match floods phase 1 with false positives
finding: 'useBacktickAutocomplete.ts lines 257-264 `filterPrefixList` matches manual-id types via `label.toLowerCase().includes(typed.toLowerCase())` — substring, not startsWith. If a project has a manual-id type with label `Code` and a regular prefix type with `CODE-`, typing `c` matches both; typing `co` still matches both (the manual `Code` via substring `co`, the prefix `CODE-` via startsWith `CO`). That''s fine. But typing `de` matches `Code` (substring `de` in `Code`) AND `DEC-` (startsWith `DE`) — the manual entry pollutes results that the user thinks are filtered to decisions only. Worse: a manual type labeled `Background-Document` would match typed `ck` via substring. The substring semantics is a UX trap: prefix list narrowing should feel like a filter, not a fuzzy search. Fix: change to `startsWith` on label, OR rank substring matches lower (push them to the end), OR show them only when user explicitly clicks a `Manual...` entry expander.'
severity: significant
resolution: Manual-type filter uses startsWith on the label (case-insensitive) instead of includes. A type labeled 'Code' no longer surfaces for typed 'de'.
status: addressed
---
