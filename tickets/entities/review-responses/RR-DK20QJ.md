---
id: RR-DK20QJ
type: review-response
title: No SSE test with only the access log; weak stderr test
finding: SSE with access log and Debug off was untested; TestNewAccessLogger_Stderr only checked non-nil.
severity: minor
resolution: Added AccessLogSSE; removed the stderr test.
status: addressed
---
