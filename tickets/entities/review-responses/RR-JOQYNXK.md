---
id: RR-JOQYNXK
type: review-response
title: The re-load fault test does not show other rules are kept, and CheckRuleFull errors are untested
finding: 'Code review: TestAnalyzeValidations_ResolveFaultNamesTheRule''s second rule had no violators; the ruleNotChecked path after CheckRuleFull had no test.'
severity: minor
resolution: The fault test now fails one rule's batch and asserts the other rule's two violators. TestAnalyzeValidations_RuleErrorNamesTheRule wraps the validator to fail one rule.
status: addressed
---
