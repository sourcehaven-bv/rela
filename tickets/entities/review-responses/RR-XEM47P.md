---
id: RR-XEM47P
type: review-response
title: Guide overstates what lowers and its cost
finding: Docs said any conjunction of equalities and related() lowers and costs the same at any size; omitted is_current_user, empty literals, repeated attributes, and that the rest of the request (q, condition, relation filter, non-string sort) must also push.
severity: minor
resolution: Guide lists the three accepted leaf shapes, the one-equality-per-property rule, and names the request shapes that keep the slower path.
status: addressed
---
