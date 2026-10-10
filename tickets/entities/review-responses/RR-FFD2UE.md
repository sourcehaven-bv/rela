---
id: RR-FFD2UE
type: review-response
title: Duplicate Vue keys for two counts of one relation
finding: The key was the relation name, which collides for an outgoing and an incoming count of the same relation.
severity: minor
resolution: Fixed. countKey uses relation:direction, or 'comments'.
status: addressed
---
