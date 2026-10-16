---
id: RR-PRO7ON
type: review-response
title: 'viewKindTraverse duplicates ViewConditionTraverse'
finding: 'The two strings have no test keeping them in sync.'
severity: nit
reason: 'Same pattern as viewKindList/viewKindAction; arch-lint keeps dataentry from importing conditionlint. A drift fails the view with errTraverseConditionUnavailable, which TestExecuteViewWithCondition catches.'
status: wont-fix
---
