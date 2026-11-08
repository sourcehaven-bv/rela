---
id: RR-5PV25TX
type: review-response
title: NewInMemory loads real host config (mail/AI/secrets)
finding: NewInMemory falls back to root/.rela host config; a world can start a live SMTP outbox and use real AI or HTTP credentials.
severity: significant
reason: TKT-NRNDGZR (part 2) adds WithMailTransport/WithHTTPClient/WithAIProvider for NewInMemory; the brief keeps those seams out of part 1. The godoc states that host config is still read from disk.
status: deferred
---
