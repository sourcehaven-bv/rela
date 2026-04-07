---
id: RR-91AX
type: review-response
title: 'S6: comment promised ''per-handler context deadlines'' that did not exist'
finding: cmd/rela-server/main.go set WriteTimeout=0 with a comment claiming slow-write protection was provided by IdleTimeout AND per-handler context deadlines. The first claim is wrong (IdleTimeout only fires between requests, not during one). The second claim is aspirational — no handler actually wraps r.Context with a deadline.
severity: significant
resolution: 'Rewrote the comment to honestly acknowledge the trade-off: a slow-reading client can hold a goroutine open. Documented that on a loopback bind this risk is limited to local processes; if you opt into LAN access via --bind, see docs/security.md for residual exposure. Per-handler deadlines were considered but deferred — adding them touches every mutating handler and is mechanical. Filed as future work in security.md residual-risks. Actually implementing them is tracked outside this ticket.'
status: addressed
---
