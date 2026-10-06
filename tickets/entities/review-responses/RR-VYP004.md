---
id: RR-VYP004
type: review-response
title: 'Bare-key deletes in EntityDelete and EntityRenamed'
finding: 'EntityDelete and EntityRenamed still delete the bare key for documents written before per-face keys.'
severity: nit
resolution: 'Not changed.'
reason: 'The bare key is also the default face key, and a downgraded build can still write such documents, so the delete stays useful.'
status: deferred
---
