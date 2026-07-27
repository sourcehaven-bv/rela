---
id: RR-73CC
type: review-response
title: No negative test for the role-relation-conferred visibility path
finding: 'Independent of the fix: there was no test where a visible: grant belongs to a role conferred by a role_relation edge (or inherit_roles_through), the edge is live, and the historical read is asserted to hide the field. Without it the fail-closed guarantee for that path could silently regress — the genuinely-absent scenario in the acceptance list (distinct from the deferred relation-history scenario 6).'
severity: significant
resolution: 'Added TestHistoryRedaction_LocalRoleConferred_FailsClosed using the buildPolicyApp harness (real acl.NewStoreGraph) with a role_relations: {owns: {confers: owner}} policy and a live alice--owns-->TKT-001 edge: live read exposes the field, historical read hides it. Plus affordances-level TestHistorical_TypeLevelClosedWorld_EmptyRoleSet pins the empty-role-set fail-closed and TestHistorical_NoVisiblePolicyForType_MarkerInert pins non-over-firing.'
status: addressed
---
