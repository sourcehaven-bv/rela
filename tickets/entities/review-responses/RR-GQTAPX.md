---
id: RR-GQTAPX
type: review-response
title: requestFor reused a ctx Request on an incomplete identity match
finding: The reuse guard compared User and Tool only, while Request.ceiling is compiled from PrincipalType and Scopes and Globals from Roles. Two principals sharing User+Tool but differing in verified claims carry different ceilings; a stale wider one would be reused. Raised by both the code and the security review.
severity: critical
resolution: requestFor compares the whole principal with principal.Equal (identity plus every verified claim). requestreuse_test gains a same-user/different-claims subtest asserting a fresh scope is opened, and the different-principal subtest asserts the walk ran.
status: addressed
---
