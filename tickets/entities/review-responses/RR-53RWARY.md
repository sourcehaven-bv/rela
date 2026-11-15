---
id: RR-53RWARY
type: review-response
title: Relation target field is ambiguous
finding: One field meant a mapping name or an entity type.
severity: minor
resolution: Split into target (a mapping name) and target_type plus target_key. Duplicate keys on a target_type lookup are a record error.
status: addressed
---
