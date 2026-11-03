---
id: RR-0BZDKN
type: review-response
title: Background paths run dataentry gates with an allow-all read gate
finding: 'readGateFromContext falls back to nopReadGate when no gate is attached. Only the HTTP middleware attaches one. Scheduler and background automation jobs bind only an acl.Request; so list rows, permission: and the elevated check would all pass for any principal.'
severity: critical
resolution: 'Plan: the render core always runs withACLEnforced and attachBoundReadGate(ctx, acl) first and fails closed on error. ACL acceptance test runs from a background automation and a schedule under a restricted principal.'
status: addressed
---
