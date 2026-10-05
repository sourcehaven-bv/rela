---
id: RR-VX8OYQ
type: review-response
title: Method logged uncapped
finding: '[security] r.Method is client-controlled up to the 1 MB header limit and was logged at full length, unlike the path.'
severity: minor
resolution: Methods over 32 bytes are logged as OTHER (test AccessLogCapsMethod).
status: addressed
---
