---
id: RR-M6KAHR
type: review-response
title: Cards/list and nested paths lack a rendering test
finding: Only the entry section has a rendering test; inline enums are untested.
severity: minor
resolution: Added an inline-enum case to PropertyDisplay.test.ts. Cards/list routing is pinned by viewRouting.test.ts; the widgets share the Badge lookup covered by the entry tests.
status: addressed
---
