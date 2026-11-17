---
id: RR-CJ57B4Z
type: review-response
title: 'Trigger ACL check: grant-key matching copied from acl'
finding: grantKeyApplies was a copy of the unexported acl helper.
severity: minor
resolution: acl exports GrantKeyApplies and the checker uses it. mayRead and surelyReads answer a may/surely question acl has no helper for, so they stay local.
status: addressed
---
