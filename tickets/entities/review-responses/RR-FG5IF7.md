---
id: RR-FG5IF7
type: review-response
title: Counting drops SoftDeleteProvider
finding: Counting embeds store.Store without forwarding SoftDelete, so delete takes a non-production path.
severity: significant
resolution: 'Plan: forward SoftDelete() in storetest.Counting.'
status: addressed
---
