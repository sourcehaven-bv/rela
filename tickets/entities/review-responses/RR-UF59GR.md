---
id: RR-UF59GR
type: review-response
title: 'Minor cleanups: dead min() in retryDelay, TestRetryDelay lacks t.Run, errBoom placement, doc log sample'
finding: '(a) retryDelay''s trailing min(delay, maxRetryDelay) is unreachable: the in-loop early return means delay < maxRetryDelay always holds there. (b) TestRetryDelay is table-driven without t.Run subtests or a name field, against the CLAUDE.md convention. (c) errBoom is declared mid-file between two helpers. (d) The retry log sample in the guide omits duration and retry_at, which the real recordFailure emits.'
severity: nit
resolution: (a) retryDelay rewritten in closed form with a derived maxLadderSteps bound, removing the dead min() and the loop; the bound is computed from baseRetryDelay/maxRetryDelay so retuning them cannot desynchronise it. (b) TestRetryDelay now uses t.Run subtests with names, plus added negative-count and 1<<40 overflow cases. (c) errBoom moved to the top with the other helpers. (d) Guide log sample corrected to include duration and retry_at, and documents the implausible-retry WARN.
status: addressed
---
