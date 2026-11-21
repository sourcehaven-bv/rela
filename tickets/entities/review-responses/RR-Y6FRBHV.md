---
id: RR-Y6FRBHV
type: review-response
title: "Login under another space lands in the current one"
finding: "The login redirect to / let the space guard pick the current space, so /s/<other>/login opened the current space's home."
severity: minor
resolution: "Addressed: The redirect keeps the space of the address. Pinned in router/notFound.test.ts."
status: addressed
---
