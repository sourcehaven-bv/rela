---
id: RR-PTXIBO
type: review-response
title: Close doc overstates the ctx bound
finding: On postgres Shutdown ignores ctx (listener close and pool close each take up to about 1s), so Close can run past ctx.
severity: significant
resolution: Lifecycle.Close doc now says the ctx bounds the wait for handlers and that stopping the backend may take longer.
status: addressed
---
