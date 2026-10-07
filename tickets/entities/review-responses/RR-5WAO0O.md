---
id: RR-5WAO0O
type: review-response
title: fs lazy build races with writes
finding: A lazy build outside the lock can overwrite fresher entries; marked entities stay on disk; the cache file is shared by CLI and server.
severity: significant
resolution: 'The build snapshots metas under s.mu, reads files with no lock, and merges under s.mu; a write during the build lands in a notes map that wins at the merge, and buildMu serializes builds. Marked families are indexed too, and liveness is checked against s.entities at read time, so unmark needs no work. The cache is written through RootedFS; an unreadable file is skipped. Tests: TestMentions_WriteDuringBuildIsKept, StaleCacheIsReread, CacheReusedForUnchangedFile.'
status: addressed
---
