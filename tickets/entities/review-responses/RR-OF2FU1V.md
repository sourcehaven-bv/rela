---
id: RR-OF2FU1V
type: review-response
title: Cold load of / sent two sidebar requests
finding: The root guard dropped the _sidebar answer, so the next visit to / asked again.
severity: minor
resolution: The guard fills the space store with the answer, so later visits to / need no request. The Sidebar still loads its own copy once, as for /s/<space>.
status: addressed
---
