---
id: RR-0RMV
type: review-response
title: parseFilterQueryParams type signature missing null
finding: Vue Router returns LocationQueryValue|LocationQueryValue[] which includes null (for ?foo without =). Plan signature uses string|string[]. Will runtime crash on null. Use vue-router's LocationQuery type.
severity: minor
resolution: parseFilterQueryParams typed against vue-router's LocationQuery. Null/undefined/empty values are skipped explicitly.
status: addressed
---
