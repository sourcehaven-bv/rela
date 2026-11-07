---
id: RR-YE7U6YW
type: review-response
title: 'Design: per-principal limiter state and fairness'
finding: A per-principal limiter map grows without bound and one principal can take every slot.
severity: significant
resolution: Limiters are keyed on the bound principal's user id, idle entries are evicted, the map is capped, each principal holds at most two slots, and the rate check runs before admission.
status: addressed
---
