---
id: RR-URT0H4
type: review-response
title: begin answers before the off check and masks ACL errors
finding: With realtime off an unevaluable caller gets 403 no_principal instead of ErrRealtimeOff; any For error including a policy load failure is reported as no_principal.
severity: minor
resolution: The realtime-off check runs before access.For; ErrNoPrincipal gives no_principal and other For errors are wrapped as plain errors.
status: addressed
---
