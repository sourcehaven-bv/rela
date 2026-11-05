---
id: RR-U2A6AEJ
type: review-response
title: A truncated body is recorded as served
finding: A failed write to the client still recorded the partial body's hash.
severity: minor
resolution: servedWriter tracks a failed write and recordServed skips the record.
status: addressed
---
