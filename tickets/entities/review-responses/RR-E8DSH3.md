---
id: RR-E8DSH3
type: review-response
title: Stale-value check ignores group_by and rows
finding: valueMatches cannot tell two cards with the same title, display and list apart.
severity: nit
reason: Two cards on one dashboard with the same title, display and list that also swap places in one reload is not a realistic edit; the check exists to catch index shifts.
status: wont-fix
---
