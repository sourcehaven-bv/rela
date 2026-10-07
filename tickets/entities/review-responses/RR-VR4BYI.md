---
id: RR-VR4BYI
type: review-response
title: Restore can revive owning edges that break the rules
finding: A restored owner could bring back an edge to a child that now has another owner.
severity: minor
resolution: checkRestoredOwning refuses with ErrOwningRule before any Unmark. TestRestore_RefusesOwningEdgeTheLiveGraphForbids.
status: addressed
---
