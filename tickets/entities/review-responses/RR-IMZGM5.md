---
id: RR-IMZGM5
type: review-response
title: Close never uninstalled the observer; a closed store kept hashing into a dead LRU
finding: New installed echoes.Recorded on the FS and nothing removed it, so after Close the FS held a strong reference to the closed store's 4096-entry LRU for the SafeFS's lifetime, and a sequentially reused FS fed the wrong tracker.
severity: minor
resolution: OnPostWrite returns a remover; fsstore keeps it as unwireEchoes and Close calls it. Pinned in TestTwoStoresOnOneFSKeepTheirOwnEchoes (a write after Close is not recorded).
status: addressed
---
