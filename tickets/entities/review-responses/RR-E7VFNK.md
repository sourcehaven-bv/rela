---
id: RR-E7VFNK
type: review-response
title: Ceiling-only policies now run the policy resolver for every principal
finding: 'Design review: choosing the policy-backed resolver for a ceiling-only policy also applies to unattenuated principals. They pay per-entity role resolution on reads, and a path without a bound acl.Request now fails closed (denyAllFields) instead of passing under Nop.'
severity: minor
reason: Intended. It is the same cost and the same fail-closed contract every policy with role field blocks already has; with no role blocks the verdicts are empty (TestPolicyResolver_NoAffordanceBlocks_PermissiveWire), so the wire for unattenuated clients is unchanged.
status: wont-fix
---
