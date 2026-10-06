---
id: RR-Q51NXA
type: review-response
title: Record skipped on handler panic
finding: The log call ran after next.ServeHTTP without defer, so panicking requests were missing.
severity: minor
resolution: Record written from a defer with panic=true and status 500 when no response was written; the panic still propagates (test RecordsPanic).
status: addressed
---
