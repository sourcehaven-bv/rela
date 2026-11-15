---
id: RR-ZD7JAYO
type: review-response
title: BodyWatch checks only the next layer for a native header pager
finding: headerPageOf asked b.Store whether it has a HeaderPager. Counting and Breadth always do; so BodyWatch(Counting(x)) took the native route even when x had none and Counting's fallback read bodies BodyWatch never saw.
severity: minor
resolution: headerPageOf now requires both the next layer and store.Innermost to be HeaderPagers. TestBodyWatch_HeaderPageCountsFallbackBodies pins native and fallback with and without Counting in between.
status: addressed
---
