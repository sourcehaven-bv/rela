---
id: RR-EON7OD
type: review-response
title: AC 9 test proves only that the example parses
finding: 'TestDocsExample_Branching (internal/realtimecfg/docs_example_test.go:114-146) asserts no problems and no warnings plus labels and props; adding style: primary to Approve would pass even if the key were decoded and dropped. Add an assertion that Approve''s Options.Style is primary and Revise''s is empty, and add TestDocsExample_TypeButton for the new Allow and Abandon example that asserts MessageType.Button for both (confirm text folded, style danger). "The prose is reviewed" is not an acceptance test; move it to the docs checklist.'
severity: minor
resolution: TestDocsExample_Branching now asserts Approve Options.Style primary and Revise empty, and new TestDocsExample_DecisionButton asserts MessageType.DecisionButton for Allow and Abandon. The prose review moves to the docs checklist.
status: addressed
---
