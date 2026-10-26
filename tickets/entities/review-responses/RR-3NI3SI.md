---
id: RR-3NI3SI
type: review-response
title: PrincipalReads and delete support are redundant
finding: 'Policy.Validate already requires that every role granting update also covers read of the type (validateVerbReadCoverage, internal/acl/policy.go:1424-1449). PrincipalReads compares on the type half the same way (roleGrantsRead, readquery.go:196), so it can never fail when PrincipalGrants(update) passed, and it is not face-aware either. Delete is unused by any output (plan:333-334). Plan change: drop PrincipalReads and the delete branch, and cite the policy invariant. Or make the read check face-aware through roleReadFaces if the intent is to catch `read: [t@a]` with `update: [t@b]`.'
severity: nit
resolution: PrincipalReads and delete dropped, citing validateVerbReadCoverage (update implies read) and that no output deletes.
status: addressed
---
