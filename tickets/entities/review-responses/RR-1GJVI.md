---
id: RR-1GJVI
type: review-response
title: No fuzz test for resolve — security-critical validator
finding: resolve will become the barrier for all high-level write paths. 12 unit cases are not enough for a CodeQL-critical choke point.
severity: minor
resolution: 'Added FuzzResolve with 13 seed inputs asserting two invariants on accepted keys: (a) filepath.Rel result never starts with ''..'' or ''..''+sep, (b) resolved path equals root or has root+sep prefix. 10-second local run passed 2.9M executions with no escapes.'
status: addressed
---
