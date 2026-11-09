---
id: RR-6VPNRIS
type: review-response
title: Multi-type span listed twice when local fingerprint is too large
finding: prepare() listed a whole multi-type span on a local ErrRangeTooLarge; a later split by type listed each type again, doubling local cost and LocalElements.
severity: significant
resolution: prepare() splits a multi-type span by type before listing anything. TestDiff_LocalTooLargeMultiTypeListsOnce pins 5000 local elements listed once.
status: addressed
---
