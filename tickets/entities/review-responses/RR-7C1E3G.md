---
id: RR-7C1E3G
type: review-response
title: PropertyDef.type TS union omits custom-type names
finding: def.type is checked against customTypes at runtime, but the TS union for PropertyDef.type only lists built-in type names — an off-schema runtime value the code relies on.
severity: nit
reason: Pre-existing divergence that labelsForProperty already relies on identically; the fix inherits rather than introduces it. Widening PropertyDef.type or adding a customType field is a cross-cutting types cleanup out of scope for this cosmetic bug fix.
status: wont-fix
---
