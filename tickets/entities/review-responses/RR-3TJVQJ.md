---
id: RR-3TJVQJ
type: review-response
title: Hyphenated relation names dropped by PROPERTY_NAME_RE on deep-link parse
finding: parseFilterQueryParams runs the key's relation name through PROPERTY_NAME_RE = /^[a-zA-Z_][a-zA-Z0-9_]*$/ (filters.ts:84,105). A relation whose name contains a hyphen (e.g. test-covers, which exists in the issues metamodel) makes a filter[test-covers]=X deep link silently dropped on parse. Pre-existing constraint, but making relation filters first-class UI surfaces it. atlas's verantwoordelijk_voor is safe (underscore), but the widget is generic.
severity: significant
resolution: 'Accepted as a known limitation for v1: relation filter_controls whose relation name is not a valid identifier ([a-zA-Z_][a-zA-Z0-9_]*) will not round-trip via deep link. atlas relations (verantwoordelijk_voor, belongs_to) are all underscore-safe. Documented in plan; widening PROPERTY_NAME_RE to allow hyphens for relation keys is a possible follow-up, out of scope here. Implementation should verify the in-scope atlas relations are identifier-safe.'
status: addressed
---
