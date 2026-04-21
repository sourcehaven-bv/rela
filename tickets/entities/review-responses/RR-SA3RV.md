---
id: RR-SA3RV
type: review-response
title: 'Leverage: OpenForWrite should return an AtomicWriter type'
finding: Reviewer suggests OpenForWrite return a *AtomicWriter that does temp+rename+fsync on Close. Centralizes atomicity correctly.
severity: nit
reason: 'Pairs with review finding #1 (streamToFile durability). Tracked as future work when attachment durability becomes a user-visible concern. Scope for this PR is path validation, not atomic-write semantics.'
status: deferred
---
