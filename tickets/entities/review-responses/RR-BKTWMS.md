---
id: RR-BKTWMS
type: review-response
title: Two definitions of empty
finding: IsEmptyValue treats {} as empty; propmatch and SQL isEmpty do not (D10).
severity: minor
resolution: '{} is now refused by validation; one emptiness definition. Test: TestIsEmptyValue compares with propmatch.'
status: addressed
---
