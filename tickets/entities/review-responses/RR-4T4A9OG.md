---
id: RR-4T4A9OG
type: review-response
title: Scheduler reorders delivery but not execution
finding: The plan schedules only response delivery. EntityDetail's contentAutoSave and each SectionEditForm have their own save chains, so two PATCHes can be in flight and the server may run them in either order. Delivery-only scheduling cannot produce that order.
severity: significant
resolution: 'The adapter schedules twice: once before the fake executes a request and once before it delivers the response.'
status: addressed
---
