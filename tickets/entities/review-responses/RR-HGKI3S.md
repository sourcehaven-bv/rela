---
id: RR-HGKI3S
type: review-response
title: propertyElements duplicated propertyContains twenty lines away
finding: propertyElements repeated the same four-case type switch as propertyContains in the same file, differing only in nil handling — and that divergence was a real, unintended behavioral difference. The commit message claimed the dynamic filter[...] path 'now agrees with the static one', but that agreement was coincidental rather than structural.
severity: significant
resolution: 'propertyContains is now a one-liner over the shared helper: `return anyElement(prop, func(el string) bool { return el == value })`. The static config-authored filters: path and the dynamic filter[...] path are the same code, so the claimed agreement is structural. This also fixed the nil divergence for both call sites at once.'
status: addressed
---
