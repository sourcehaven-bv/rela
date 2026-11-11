---
id: RR-9J0UYQ2
type: review-response
title: File backend writes meta per message and leaves partial batches on crash
finding: filechannels Append writes meta.yaml before the log; looping writes meta N times and a crash leaves a partial batch.
severity: minor
resolution: 'Plan: the file backend builds all records in memory, writes meta once and records once; the Store doc states crash behaviour; a crash test like jobcrash_test.go.'
reason: Meta is written once per batch. A batch across a segment boundary is still two log writes and no crash test exists; deferred with RR-JH8JPTD to a follow-up on file-backend crash repair.
status: deferred
---
