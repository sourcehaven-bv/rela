---
id: RR-GY78DHP
type: review-response
title: ACL name match over-refuses on prose
finding: Whole-word match anywhere in acl.yaml, including description prose, refuses common names.
severity: minor
resolution: 'Fail-closed on purpose: a false refusal sends the operator to flows.yaml, a false pass loosens access. Replaced by the policy-based check in TKT-CB1YV7D.'
reason: 'Fail-closed on purpose: a false refusal sends the operator to flows.yaml, a false pass loosens access. Replaced by the policy-based check in TKT-CB1YV7D.'
status: wont-fix
---
