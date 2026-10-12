---
id: RR-WU343D
type: review-response
title: Test harness cannot run blocking cases
finding: 'Architecture design review: callTool uses context.Background, and memstore cannot count subscribers.'
severity: minor
resolution: The plan adds callToolCtx on the in-memory client transport, uses WithPrincipal for ACL cases, and a fake ChangeSource for AC8.
status: addressed
---
