---
id: RR-9J0UYQ2
type: review-response
title: File backend writes meta per message and leaves partial batches on crash
finding: filechannels Append writes meta.yaml before the log; looping writes meta N times and a crash leaves a partial batch.
severity: minor
resolution: 'Plan: the file backend builds all records in memory, writes meta once and records once; the Store doc states crash behaviour; a crash test like jobcrash_test.go.'
reason: 'The file backend is the developer tier: one user with the project in git. Normal users run sqlite or postgres, which write a batch in one transaction. Crash repair for batches there is not worth its complexity.'
status: wont-fix
---
