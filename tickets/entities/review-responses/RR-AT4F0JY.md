---
id: RR-AT4F0JY
type: review-response
title: 'Design: re-parent with reject reveals hidden scoped edges'
finding: 'Refusing a re-parent of a target because some entity holds a scoped edge into it tells the caller that such an edge exists, also when its source is hidden.'
severity: critical
resolution: 'Classified as the accepted relation-rule channel in docs/acl-security.md (one bit about a readable end: whether a counted edge exists, hidden ones included). The refusal uses one fixed message that names the relation and its within, never an entity. Documented in the ACL guide; a test with a hidden source pins that the message carries no id or title.'
status: addressed
---
