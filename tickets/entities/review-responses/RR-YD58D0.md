---
id: RR-YD58D0
type: review-response
title: StaticReadView on Policy would be a second ACL evaluator
finding: A Policy method calling ceilingFor directly bypasses Request.roleFor, the clamp point; ceilingguard_test only matches policy.Roles[ so it would not catch it.
severity: significant
resolution: Plan builds a real Request via NewDeclarative + ForPrincipal with a VerifiedFrom principal (same path as aclmap.MapPrincipalAs) and adds only (*Request).ReadsType using roleFor.
status: addressed
---
