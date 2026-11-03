---
id: RR-C91FORX
type: review-response
title: Context strip accessibility under-specified
finding: 'aria-current on a span in a p is unreliable; strip and expanded list would both carry aria-current; channelSessionStep renders ''X, next'' not ''Next state: X''; current has no spoken prefix; visible labels not aria-hidden.'
severity: significant
resolution: Strip is an ol labelled by a new sessionContext key; each item has hidden text from channelSessionContextItem (previous/current/next) and an aria-hidden visible label; the strip is removed while the list is unfolded. AC 9.
status: addressed
---
