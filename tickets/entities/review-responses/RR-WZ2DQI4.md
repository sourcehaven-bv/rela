---
id: RR-WZ2DQI4
type: review-response
title: directOnly would widen a negated predicate
finding: '[security] Stripping the inheritance expansion from a Negate predicate turns ''no ancestor has the edge'' into ''the entity lacks it'', a superset. No ACL path sets Negate today.'
severity: minor
resolution: directOnly returns a negated predicate unchanged, so foldHop refuses it; documented.
status: addressed
---
