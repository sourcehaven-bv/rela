---
id: RR-7TOC2CT
type: review-response
title: FlowWaiting keys collide for fork branches; flows.ts doc wrong
finding: Rows were keyed by to and guard, which collide for forks; the doc said `to` is the first fork target.
severity: nit
resolution: Rows are keyed by index; the doc says `to` is empty for a fork.
status: addressed
---
