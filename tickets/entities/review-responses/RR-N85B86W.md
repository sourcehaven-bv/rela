---
id: RR-N85B86W
type: review-response
title: 'Design: ETag and element hash differ for a stored value under a roll-up name'
finding: servedETag hashes the stored row minus hidden keys; it keeps a stale value stored under a roll-up name which the element view drops.
severity: minor
reason: The element view drops the never-served value on purpose (hashing it would make an oracle for a value no reader is served). Changing the ETag changes every If-Match for such rows and belongs to dataentry; the exception is documented in the reconcile package doc.
status: deferred
---
