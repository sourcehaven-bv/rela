---
id: RR-L9R82V0
type: review-response
title: Event index duplicates selects without a test
finding: mayFire in compile.go repeats the event-key logic of Engine.selects as a superset. A key added to selects but not to mayFire would silently never fire, and the golden tests only cover today's keys.
severity: minor
resolution: 'Added TestMayFireCoversSelects: 128 trigger-key combinations against 8 events assert that selects implies mayFire. The mayFire doc names the test.'
status: addressed
---
