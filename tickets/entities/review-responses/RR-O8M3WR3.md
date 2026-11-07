---
id: RR-O8M3WR3
type: review-response
title: 'Design: identity errors need explicit mapping'
finding: acl.ErrNoIdentity would become a 500.
severity: minor
resolution: Errors go through writeNoIdentity (401) and writeInternalProblem; a test pins the unbound case.
status: addressed
---
