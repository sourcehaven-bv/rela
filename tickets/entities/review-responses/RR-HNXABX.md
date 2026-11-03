---
id: RR-HNXABX
type: review-response
title: 'Plan gaps: limits, identity lifetime, PUT registration, path parity, index lag'
finding: No per-principal caps; stream outlives JWT expiry; PUT route would fall through to dynamic routes; subscribed path could differ from fetched path; search index may lag a q= re-read; re-read error policy unspecified.
severity: minor
resolution: Added per-principal cap (429), PUT on the inner mux with CSRF/JWT and MaxBytesReader, shared URL builder with a parity test, index-lag check as a risk with a fallback, error policy keep-old-hash-and-log. JWT expiry recorded as an accepted residual in acl-security.md plus a follow-up, since the principal carries no exp.
status: addressed
---
