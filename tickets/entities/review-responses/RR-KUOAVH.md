---
id: RR-KUOAVH
type: review-response
title: A10 checks assignment values but not assignment keys
finding: A10 (tier_a.go) flags whitespace on p.Assignments[key] (the role name) but never on key itself. The key is the member/group ID matched in computeGlobals (policy.Assignments[m]). A key 'admins ' silently matches no member — the same silent foot-gun A10 exists to catch. Add a key whitespace check.
severity: minor
resolution: A10 now also checks assignment KEYS for whitespace (the member/group ID). Added TestAudit_A10_AssignmentKeyWhitespace.
status: addressed
---
