---
id: RR-UXWZ2I
type: review-response
title: fs mention stamp recorded on write was never persisted
finding: noteMentions stat-ed the file under the lock to record a stamp, but the cache is saved only by the build, so the stat was wasted and its comment was false.
severity: minor
resolution: noteMentions no longer stats; a noted entry carries no stamp, so a later process rereads that file. The build saves the cache before merging notes. Comments say so.
status: addressed
---
