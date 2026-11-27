---
id: RR-KUB90G2
type: review-response
title: No ACL tests for relation triggers
finding: Every test used acl.NopACL.
severity: significant
resolution: relation_triggers_acl_test.go uses a real acl.Declarative policy with refused and allowed callers on both faces.
status: addressed
---
