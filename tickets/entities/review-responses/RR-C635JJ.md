---
id: RR-C635JJ
type: review-response
title: 'Test gaps: redaction, ordered comparison, stale config'
finding: 'No test proved the condition reads the redacted entity, nor pinned ordered comparison on foreign fields or a stale config.'
severity: significant
resolution: 'Added TestViewMentions_ConditionReadsRedactedEntity, the ordered-comparison case in TestEvaluator_CompileForCollection, TestExecuteViewWithCondition_StaleProgramFails and the where-narrowing case. Worlds are covered by the same `found` slice the where: world test pins.'
status: addressed
---
