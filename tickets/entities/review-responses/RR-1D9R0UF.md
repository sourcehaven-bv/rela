---
id: RR-1D9R0UF
type: review-response
title: Ignoring slot ok is fail-open in shape
finding: Callers discard slot's ok; a token the run does not carry gives a zero instance that reads as initial.
severity: minor
resolution: 'Fixed in TKT-FWQZGAM: settleStep returns early when slot reports no token or the token has no pending mark, so a missing token never reads as a settled step.'
reason: Every caller gets its token from pendingToken or tokenAt, which only name tokens the run carries. Hardening is tracked with the worker in TKT-FWQZGAM, which revisits these paths.
status: addressed
---
