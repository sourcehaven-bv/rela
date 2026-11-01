---
id: RR-PKPP2FN
type: review-response
title: 499 rule leaves 500s written to a dead client
finding: With 499 only when nothing was written, any path that still writes a 500 logs status=500 canceled=true, indistinguishable from a fault. Define precedence with panic.
severity: significant
resolution: 'Plan changed: a request whose context was canceled before the handler returned logs status=499 canceled=true, with written_status when a status was written; panic keeps 500 panic=true; SSE paths are excluded.'
status: addressed
---
