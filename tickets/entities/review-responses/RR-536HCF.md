---
id: RR-536HCF
type: review-response
title: Schema key touchpoints missing
finding: validTopLevelKeys, include.go partialMetamodel, the configedit lock list and acl map were not in the plan.
severity: minor
resolution: 'Plan: add the key to all of them with a root-only error in include.go.'
status: addressed
---
