---
id: RR-30U7K
type: review-response
title: IsSoft belongs to metamodel package, not workspace
finding: 'Plan calls choice ''marginal''. It isn''t. Classifier is property of error category, not policy of who''s calling. Every consumer (workspace, future per-edge, MCP if needed) needs the answer. Workspace placement = re-implementation drift OR import bloat. ''Tests can''t easily verify from metamodel package'' objection is wrong — testing a method on its own struct in its own package is the easiest case. Metamodel owns categories; categorizing them is same level of abstraction. ''Couples to policy'' confuses ''knows what kind of error'' with ''decides HTTP code'' — only the latter is policy. Recommendation: define func (e *ValidationError) IsSoft() bool in internal/metamodel/validation.go. From design-review F7.'
severity: minor
resolution: IsSoft() placed on *metamodel.ValidationError in internal/metamodel/validation.go. Test in validation_test.go (table test enumerating all categories). Workspace boundary calls err.IsSoft() to partition. Categorization is a property of the error category, not workspace policy.
status: addressed
---
