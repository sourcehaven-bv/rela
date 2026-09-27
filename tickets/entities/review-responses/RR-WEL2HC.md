---
id: RR-WEL2HC
type: review-response
title: refusingTraversalGate comment implies fail-closed for a missing gate
finding: readGateFromContext returns nopReadGate when no gate is on ctx, so a missing gate is ungated like every other read.
severity: nit
resolution: Comment rewritten to say so.
status: addressed
---
