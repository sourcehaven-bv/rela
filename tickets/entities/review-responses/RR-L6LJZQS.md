---
id: RR-L6LJZQS
type: review-response
title: Store fault must not fall back to title mode
finding: 'A failed id lookup read as not served turns a valid id into a title that matches nothing; for ne that admits every row. Fix: return the lookup error (errListLoad) and look up only after the field gate passes.'
severity: minor
resolution: resolveRelationTargets runs after the field gate and returns a lookup error as errListLoad.
status: addressed
---
