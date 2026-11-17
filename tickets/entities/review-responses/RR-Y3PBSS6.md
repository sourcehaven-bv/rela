---
id: RR-Y3PBSS6
type: review-response
title: Abstract-type expansion and single-type checks missing
finding: namespaces.go expands on.entity to variants and singleTypeErrors rejects an abstract create_entity.type; without them a trigger on an abstract type matches nothing.
severity: significant
resolution: 'Plan: both move into the flowcfg trigger checks with tests.'
status: addressed
---
