---
id: RR-7IOBDB
type: review-response
title: Default-value changes flip the shape hash; classifier tier unstated
finding: ShapeProjection includes property defaults (unlike RenderProjection), so editing a default changes the shape hash — but a default affects only future creates, never stored data. The classifier table doesn't name this case; if it fell through to needs-migration the gate would demand a pointless migration for a benign edit. Must be explicitly tiered additive (auto-adopt, silent).
severity: minor
resolution: 'Amendment A7: default-value-only changes are explicitly listed in the additive tier of the classifier table (they affect only future creates, never stored data) and auto-adopt silently. Covered by the AC1 cosmetic/shape hash table test.'
status: addressed
---
