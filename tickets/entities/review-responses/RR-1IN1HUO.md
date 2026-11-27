---
id: RR-1IN1HUO
type: review-response
title: Blocking constructor and prototype drops legal names
finding: 'filters.ts: only __proto__ replaces a plain object''s prototype on assignment. constructor and prototype are metamodel-legal names and harmless as own keys, so blocking them drops valid deep links.'
severity: minor
resolution: Only __proto__ is rejected now. Tests keep constructor and prototype as legal names.
status: addressed
---
