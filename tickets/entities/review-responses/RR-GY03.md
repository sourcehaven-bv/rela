---
id: RR-GY03
type: review-response
title: 'Migration risk: existing custom type named rrule would be shadowed'
finding: Adding rrule as built-in shadows any existing custom type with that name. Should detect and warn during migration.
severity: minor
reason: Extremely unlikely edge case (custom type named 'rrule'). Will add a migration detection step in a follow-up if needed. The loader already prevents custom types from shadowing built-in names — adding rrule as built-in means loader.go line 124 will reject any custom type also named rrule.
status: deferred
---
