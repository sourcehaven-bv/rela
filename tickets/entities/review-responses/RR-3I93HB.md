---
id: RR-3I93HB
type: review-response
title: Test table missed every edge case the review found
finding: The list-filter tests covered the happy path well but had no case for nil property, absent property, empty CSV, comma-in-value, whitespace, mixed-type list, or ordered-operator-on-list — six of which the reviewer found bugs in by probing manually.
severity: minor
resolution: Added TestV1FilteringListProperty_EdgeCases covering the comma-bearing value under both in and ne, the nil-vs-"<nil>" case, empty-eq against a null property, and ordered-operator-on-list. Also folded []any into the matchList table so every operator runs against both []string and []any (previously OpNotEqual was untested on []any). Mixed-type and whitespace behavior was verified by probe and is documented rather than pinned, since those are pre-existing semantics this bug does not define.
status: addressed
---
