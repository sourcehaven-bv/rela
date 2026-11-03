---
id: RR-VR1DYU
type: review-response
title: Fixture has no field or relation grants
finding: acl.yaml declares no fields/options/relations/relation_grants, so verdicts and affordances are always permissive; PR 5 is unguarded.
severity: critical
resolution: acl.yaml has field, option and relation grants and a transition guard; PATCH of a denied field and option, the guarded transition and the protected relation DELETE are recorded (403/422).
status: addressed
---
