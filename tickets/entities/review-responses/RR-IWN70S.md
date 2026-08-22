---
id: RR-IWN70S
type: review-response
title: week alias table row still invites ISO-week mental model
finding: 'The Schedule Values table row for `week` read only ''Alias for `monday`''. That naming is the exact origin of the ISO-week confusion this bug documents: a reader writing `schedule: week` will assume week-boundary semantics. The prose fix alone treated the symptom while the vocabulary row kept teaching the wrong model.'
severity: significant
resolution: Table row now reads 'Alias for `monday` — fires on Mondays, not ISO-week change' in GUIDE-scheduled-tasks.md; docs/scheduled-tasks.md regenerated.
status: addressed
---
