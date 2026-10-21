---
id: RR-DP8KCY
type: review-response
title: Some required-field inputs send wrong values
finding: An untouched required boolean is never sent (422); datetime, rrule, list enums and references fall back to a single text field.
severity: minor
resolution: Required booleans are seeded false and lists empty; a list enum is a checkbox group sending an array; datetime uses the existing DatetimeWidget. Component tests cover boolean, list enum and datetime.
status: addressed
---
