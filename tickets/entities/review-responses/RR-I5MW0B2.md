---
id: RR-I5MW0B2
type: review-response
title: One current value does not keep the slot stable across placed pages
finding: With visible_on, a list then kanban then list loop sent the kanban suggestion as current, so the list's own suggestion fell into cooldown and the list moved on every round trip.
severity: significant
resolution: The client remembers the suggestion each page last showed (bounded at 20) and sends that page's one as current; answered suggestions are forgotten on every page. Pinned by a mutation-checked test.
status: addressed
---
