---
id: RR-EVXN
type: review-response
title: URL routing admits slashes and special chars in action IDs
finding: TrimPrefix + map lookup is mostly safe but plan doesn't specify the ID validation regex. Define it (e.g. ^[a-z0-9_-]{1,64}$) and enforce at config load time.
severity: critical
resolution: Added actionIDRegex = ^[a-z0-9_-]{1,64}$ enforced both at config load time and request time.
status: addressed
---
