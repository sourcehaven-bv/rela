---
id: RR-5RJR082
type: review-response
title: Nested settles run as a synchronous chain in the request
finding: A settle that writes a watched subject settled the next run inline, so one request could run a chain of settles.
severity: significant
resolution: settleStarted leaves a write made while settling to the flow worker (settlingKey); the chain test drives due rounds.
status: addressed
---
