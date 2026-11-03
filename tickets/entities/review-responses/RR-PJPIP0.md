---
id: RR-PJPIP0
type: review-response
title: Library messages must read the locale per call
finding: Announcements built in event handlers run outside render. A Messages object that captured the locale at provide time would announce in the old language after a switch.
severity: minor
resolution: 'Plan: localeMessages takes a getter and resolves the set on each call; a unit test switches the locale between two calls.'
status: addressed
---
