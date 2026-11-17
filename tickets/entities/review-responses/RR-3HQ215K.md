---
id: RR-3HQ215K
type: review-response
title: 'Security: acl.yaml match is textual'
finding: The regex runs over raw acl.yaml text; YAML escapes are missed and decode errors are swallowed.
severity: minor
resolution: The textual aclWriteReferences is removed; the check works on the parsed acl.Policy.
reason: Replaced by the policy-based check in TKT-CB1YV7D.
status: addressed
---
