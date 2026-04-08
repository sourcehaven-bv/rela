---
id: RR-T5RQ
type: review-response
title: Override semantics produce zero-result trap on collision
finding: 'If config has filters: status=closed and URL has filter[status]=open, both go to backend with different keys (filter[status][eq] vs filter[status]). Backend ANDs them → ZERO results, no error. AC5 wording ''URL is OR''d into nothing'' is factually wrong about how the code works. Need explicit collision detection: skip+warn, override, or reject at config-load time.'
severity: significant
resolution: useUrlFilterSync receives a staticFilterProperties getter from EntityList. On read, URL filters for blocked properties are deleted and a console warning is emitted. No collision sent to backend, no zero-result trap.
status: addressed
---
