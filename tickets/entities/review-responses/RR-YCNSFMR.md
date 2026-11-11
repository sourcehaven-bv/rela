---
id: RR-YCNSFMR
type: review-response
title: fs and memory run-write race is not tracked
finding: 'pr-guidance G-005: the fs/memory residual (run writes are not ordered against field writes) was documented but had no ticket.'
severity: minor
resolution: TKT-LU8PSKJ records it as accepted risk, citing the decision that fs gets no lock. The guide and the PLAN-9PX91CT threat model link it.
status: addressed
---
