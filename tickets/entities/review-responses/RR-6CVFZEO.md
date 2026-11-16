---
id: RR-6CVFZEO
type: review-response
title: An existing relation aborted the apply
finding: CreateRelation on an edge made while the model answered returned an error and stopped the rest of the apply.
severity: significant
resolution: ErrRelationAlreadyExists is skipped. TestApplyProposal_ExistingRelationSkipped pins it.
status: addressed
---
