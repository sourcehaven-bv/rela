---
id: RR-T7SY1M
type: review-response
title: Typed gate is not enforced down to the evaluators
finding: gatedListQuery can be built by a literal anywhere in dataentry, and applyV1Filters, applyV1Sorting and planListPushdown still take raw maps. Make the evaluators take gatedListQuery and add a grep guard that allows the literal only in listfieldgate.go.
severity: significant
resolution: applyV1Filters, applyV1Sorting, planListPushdown, pushdownPlan and scopedSortedEntitiesScoped take gatedListQuery. TestGatedListQuery_BuiltOnlyByTheGate parses every non-test file and fails on a gatedListQuery literal outside listfieldgate.go.
status: addressed
---
