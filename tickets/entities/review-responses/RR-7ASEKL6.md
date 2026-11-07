---
id: RR-7ASEKL6
type: review-response
title: createOutput calls ResolveAlias twice
finding: states.go calls ResolveAlias(raw.Type) in the case and again in the message; load.go computes canon once.
severity: nit
resolution: createOutput computes canon once before the switch.
status: addressed
---
