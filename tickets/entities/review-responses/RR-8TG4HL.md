---
id: RR-8TG4HL
type: review-response
title: Guard misses GetEntityState with a literal empty face
finding: GetEntityState(ctx, id, "") is the same zero-face read as GetEntity(ctx, id) spelled differently. A guard on GetEntity alone makes that spelling the path of least resistance for new code (9 production sites today).
severity: significant
resolution: The checker also counts GetEntityState calls whose third argument is the literal ""; they share the per-file allowlist.
status: addressed
---
