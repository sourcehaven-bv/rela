---
id: RR-YHJUFN
type: review-response
title: Channel type named messages cannot be subscribed live
finding: livepath checks the last segment for messages, which also matches the type segment of a one-cardinality channel named messages.
severity: minor
resolution: The messages segment is recognised only at segment 4 of a five-segment path; tests cover a type named messages.
status: addressed
---
