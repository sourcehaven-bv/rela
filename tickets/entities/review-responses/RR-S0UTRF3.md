---
id: RR-S0UTRF3
type: review-response
title: Deleted settings leak tests
finding: TestACLSettings_RelationTargetsRedactHiddenTitle and TestACLSettings_RelationTargetsDropUnreadable were removed.
severity: nit
reason: The response carries no target data; TestSettings_RelationDefsCarryTargetTypesOnly asserts no target id or title appears.
status: wont-fix
---
