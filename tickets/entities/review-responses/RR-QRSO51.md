---
id: RR-QRSO51
type: review-response
title: Up to three full serializations per guarded PATCH
finding: currentVersions builds the full wire entity only to hash properties/content/relations; a lost race serializes and reads faceEdges up to three times.
severity: minor
reason: Correctness is unaffected and a guarded PATCH is a single-entity write. A strip-and-redact-only projection can replace the full serialization if profiling shows the cost.
status: deferred
---
