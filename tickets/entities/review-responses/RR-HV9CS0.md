---
id: RR-HV9CS0
type: review-response
title: Refresh lock unbounded and not reentrancy-safe
finding: No acquire timeout, re-entrant deadlock, release depends on Lua unwinding.
severity: significant
resolution: 'Plan R1: refresh in Go with 60 s acquire timeout and deferred release; no Lua callback.'
status: addressed
---
