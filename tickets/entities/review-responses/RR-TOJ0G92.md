---
id: RR-TOJ0G92
type: review-response
title: principal_property field can be mapped
finding: Mapping the acl.yaml principal_property lets the source decide which login maps to which local entity.
severity: minor
reason: The mapping is operator-authored config and Compile does not load acl.yaml; a compile-time refusal belongs with the ACL-aware checks of TKT-6GRZHXD.
status: deferred
---
