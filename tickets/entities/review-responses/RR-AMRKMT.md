---
id: RR-AMRKMT
type: review-response
title: 'Single SafeFS observer slot: a second store on the same FS evicted the first''s recorder while echoesWired stayed true'
finding: SafeFS.OnPostWrite replaced the previous observer, so fsstore.New on a second store over one FS silently unsubscribed the first store's echo tracker; the first store's echoesWired flag still reported success and StartWatching ran a watcher with dead echo suppression — the exact BUG-S24X52 symptom, reconstituted. Reproduced by the reviewer with a two-store probe (duplicate EventEntityUpdated). Production is safe only by call-site accident (one OpenStore per factory; desktop mints a fresh SafeFS per project).
severity: significant
resolution: 'The post-write hook is now a fan-out registry shared by SafeFS and MemFS (internal/storage/postwrite.go): OnPostWrite appends and returns a remover, nothing evicts. fsstore installs an unexported recordSelfWrite scoped to its own entities/relations dirs, so stores over different roots on one FS record only their own writes. Pinned by TestTwoStoresOnOneFSKeepTheirOwnEchoes and TestSafeFS_OnPostWrite_FanOutAndRemove.'
status: addressed
---
