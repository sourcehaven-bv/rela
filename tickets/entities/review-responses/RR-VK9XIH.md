---
id: RR-VK9XIH
type: review-response
title: InheritThrough silently ignored when Endpoints is empty
finding: A caller passing InheritThrough + Depth with an empty Endpoints gets those fields silently ignored on both backends -- pgstore skips the endpoint-closure CTE, and the naive impl's expandSet returns nil on empty seeds then short-circuits via anyEndpoint. The backends agree, so this is not a parity bug, but the API swallows a caller error and the resulting query means something far broader than what was written. With Negate now available, 'no edge from my group-expanded set' with an accidentally-empty set flips from matching nobody to matching everybody.
severity: significant
resolution: 'Documented on store.RelationPredicate.Endpoints that an empty set means ANY endpoint and is a widening, and that InheritThrough is inert without Endpoints. The concrete security-relevant caller (the ACL read gate) now fails closed explicitly rather than relying on the predicate to narrow -- see RR-6947C1. A validator rejecting the combination was considered and deferred: the doc plus the caller-side guard covers the reachable risk, and GraphQuery has no validation seam today, so adding one is a larger change than this finding warrants.'
status: addressed
---
