---
id: RR-5PV25TX
type: review-response
title: NewInMemory loads real host config (mail/AI/secrets)
finding: NewInMemory falls back to root/.rela host config; a world can start a live SMTP outbox and use real AI or HTTP credentials.
severity: significant
resolution: 'Fixed in TKT-NRNDGZR (PR 2/4): NewInMemory defaults to a memory mail transport (mail.yaml still parsed for its recipients policy), refuses HTTP and AI with ErrRealNetwork, and serves only mail.yaml from host config with no secrets. TestNewInMemory_DefaultsKeepTheWorldOffTheNetwork shows zero connections to the configured SMTP and AI host.'
reason: TKT-NRNDGZR (part 2) adds WithMailTransport/WithHTTPClient/WithAIProvider for NewInMemory; the brief keeps those seams out of part 1. The godoc states that host config is still read from disk.
status: addressed
---
