---
id: RR-06QZPK2
type: review-response
title: History button appears late and not after an in-page edit
finding: The button is hidden until the timeline answers, so it can appear after first paint, and an edit made on the page does not show it until the next visit.
severity: minor
reason: The version sweep is debounced, so a recheck right after an edit would also find nothing. The behaviour is documented in useHasHistory.
status: wont-fix
---
