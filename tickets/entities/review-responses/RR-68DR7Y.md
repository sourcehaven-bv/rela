---
id: RR-68DR7Y
type: review-response
title: Cursor time precision is unspecified
finding: 'The cursor carries "an RFC 3339 time". Stored times are channels.NormTime (UTC, truncated to microseconds). If the cursor time is coarser, or compared without normalizing, entries entered within one second repeat or are skipped. Fix: encode RFC3339Nano in UTC at microsecond precision, NormTime the decoded value, and add a paging test with equal and sub-second `since` values.'
severity: minor
resolution: Cursor since is UTC RFC3339Nano at microsecond precision, passed through channels.NormTime on decode. AC 10 paging test adds equal and 1 µs-apart since values across page boundaries.
status: addressed
---
