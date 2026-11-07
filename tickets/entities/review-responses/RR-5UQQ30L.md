---
id: RR-5UQQ30L
type: review-response
title: 'Code: PolicyResolver.RelationMetaGated breaks the plimsoll method cap'
finding: The new method takes PolicyResolver to 41 methods; the cap is 40 and the God-object lint job fails.
severity: critical
resolution: RelationMetaGated is now a package function affordances.RelationMetaGated(r *PolicyResolver; relType); PolicyRedactor calls it.
status: addressed
---
