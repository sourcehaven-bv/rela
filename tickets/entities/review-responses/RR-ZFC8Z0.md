---
id: RR-ZFC8Z0
type: review-response
title: Audit refused policies using principal_property
finding: The synthetic audit principal went through principal_property resolution with no lookup, so acl audit failed on any policy that sets it.
severity: critical
resolution: The policy copy clears PrincipalProperty; the synthetic user is already an id. Pinned by TestClassificationFindings_PrincipalProperty.
status: addressed
---
