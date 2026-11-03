---
id: RR-X3WMLU
type: review-response
title: Per-identity ephemeral Retry-After too early
finding: The per-identity count and channel caps use the process-wide earliest expiry instead of the caller's own.
severity: nit
resolution: The per-identity count cap waits for the caller's earliest-expiring signal and the channel cap for the earliest time one of the caller's channels frees; the byte budget keeps the process-wide expiry. TestEphemeralStore_IdentityCapRetryAfter.
status: addressed
---
