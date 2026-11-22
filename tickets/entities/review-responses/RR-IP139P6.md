---
id: RR-IP139P6
type: review-response
title: Second invoices fetch is the live-view baseline
finding: The invoices list is fetched again after the live subscription PUT.
severity: minor
reason: Deliberate baseline refetch so no update between the first read and the subscription is missed; removing it needs a live protocol change.
status: wont-fix
---
