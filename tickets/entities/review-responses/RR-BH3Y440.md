---
id: RR-BH3Y440
type: review-response
title: List ignores the batch action affordance
finding: The list rendered every list.actions entry without reading _actions action:<id>, so a caller without the permission saw a button that 403s.
severity: minor
resolution: resolvedActions hides a batch action unless the list response carries action:<id> true; tested both ways.
status: addressed
---
