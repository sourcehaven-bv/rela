---
id: RR-FC1J3
type: review-response
title: Test uses non-existent property type "number"
finding: 'types_test.go uses Type: "number" but PropertyTypeInteger = "integer" — there is no "number" type. Test passes because EntityDef is constructed directly without going through validation. Cosmetic but inconsistent with the property-type contract. Fix: use "integer" (or another defined type).'
severity: minor
resolution: Replaced literal "number" with PropertyTypeInteger constant in types_test.go:398.
status: addressed
---
