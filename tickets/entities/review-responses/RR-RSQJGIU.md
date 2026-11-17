---
id: RR-RSQJGIU
type: review-response
title: '[security] Endpoint match Where is ignored'
finding: '[security] The reviewer reported that a Where on an endpoint match is ignored by the backends.'
severity: minor
reason: 'The type has no Where field: EndpointMatch is an EndpointPredicate. A change adding the check did not build, so there is nothing to ignore.'
status: wont-fix
---
