---
id: RR-G1EVIB
type: review-response
title: Hub starts its own worker goroutine
finding: 'Architecture review: start takes the concrete Bus and starts the goroutine; an exported Run would let StartWatching own it.'
severity: nit
reason: Part of the package move in TKT-H6LD1Y, where Run becomes the package API.
status: deferred
---
