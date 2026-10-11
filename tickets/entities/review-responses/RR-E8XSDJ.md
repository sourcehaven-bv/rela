---
id: RR-E8XSDJ
type: review-response
title: Renames and option removals can silently widen the ACL
finding: 'Client-baseline redact matches fields by name (acl/ceilingcompile.go:462) and row-grant when: predicates match option values by name; ValidateAgainstMetamodel checks only identity keys (acl/policy.go:1411-1465).'
severity: significant
resolution: 'Plan changed: renaming or removing a type, property, option or relation type that acl.yaml references is refused, naming the reference.'
status: addressed
---
