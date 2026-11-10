---
id: RR-3TAZGJL
type: review-response
title: flowOf compiles the snapshot on every step
finding: Each advance of an older-version run recompiles its stored definition; there is no cache.
severity: minor
reason: A cache keyed by flow and version belongs with the background worker in TKT-FWQZGAM, which is the hot caller. Manual starts and Due rounds are bounded by MaxDuePerRound.
status: deferred
---
