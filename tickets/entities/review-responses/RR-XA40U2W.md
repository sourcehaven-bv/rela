---
id: RR-XA40U2W
type: review-response
title: Missing tests for several paths
finding: No tests for the combined PATCH, multi-edge removal, negated rule on create, both ends refusing, and an edge to a gone entity.
severity: minor
resolution: Added table cases in relationrules_test.go (both ends, negated rule on create, gone far id) and relation_rule_test.go (combined PATCH, removing every edge at once, removing one of two). HTTP POST and If-Match DELETE 422 share writeRuleRefusal with the tested DELETE path.
status: addressed
---
