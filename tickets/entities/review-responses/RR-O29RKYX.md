---
id: RR-O29RKYX
type: review-response
title: Endpoint allowlist logic duplicated
finding: endpointTypeError repeated ValidateRelation's allowlist checks and codes.
severity: minor
resolution: Added metamodel.RelationEndCode; ValidateRelation and endpointTypeError both use it.
status: addressed
---
