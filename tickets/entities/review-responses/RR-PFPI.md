---
id: RR-PFPI
type: review-response
title: Static filter collision locks the whole property (no ranges)
finding: 'useUrlFilterSync''s collision check uses staticFilterProperties: Set<string> — just property names. If data-entry.yaml has filter[date][gte]=2024-01-01, the user cannot add filter[date][lte]=2024-12-31 via the URL because the entire ''date'' property is locked. The current warning text says ''locked by static config filter'' but doesn''t explain the granularity. Two options: (a) document explicitly that static filters lock the whole property and update the warning to clarify this, (b) upgrade the collision check to (property, operator) keys so [gte] and [lte] can coexist. (a) is sufficient for v1; file (b) as a follow-up.'
severity: significant
resolution: useUrlFilterSync console.warn message expanded to explicitly state that static filters lock the WHOLE property, not individual operators, and points users to data-entry.yaml as the place to define ranges that combine with a static bound. docs/data-entry.md URL Sync section adds an 'Important' note making the whole-property granularity explicit and offering the workaround. Test updated to assert both the property name and the 'whole property' phrasing are in the warning. Per-operator collision remains an open enhancement (tracked separately if needed).
status: addressed
---
