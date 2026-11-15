---
id: RR-CJTDXS2
type: review-response
title: Non-string value in a string or enum property is judged differently
finding: filter.MatchAll returns an error and matches no row for a number in a string property; the store compares its text form and can match.
severity: minor
resolution: 'Addressed: exactSearchPredicate''s doc states the plan can return such a row where the Go path would not, as the list pushdown can.'
status: addressed
---
