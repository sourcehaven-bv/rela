---
id: RR-Q0GXDE
type: review-response
title: Dirty in-memory shards outlive leadership and overwrite newer KV state, so posts are repeated
finding: 'internal/realtime/trigger.go:627-644 (round.shard reads t.dirty first) and 924-956 (save re-Puts every dirty shard); Drop() at 188-192 does not clear dirty. Scenario: A''s shard save fails (brief KV blip), so A keeps the shard in t.dirty. B then runs rounds and updates the same shard in the KV (enters, leaves). A runs again (the lock alternates), and shard() returns A''s stale dirty copy instead of the KV value. A re-evaluates against stale data, which reposts B''s enters and leaves, and save() overwrites B''s newer shard. The same happens after a reconcile on takeover. The log message at 953 admits other processes may repost but not that this process clobbers their records. Fix: only trust dirty while this process has led without a break. Clear dirty (and lastPost, failures, deferred) in Drop()/on takeover, or compare a version/etag before using it. With the lifetime leader lock this reduces to "clear on losing the lock".'
severity: significant
resolution: Triggers.Reset clears all leader state, including unsaved shards, deferred ids, cooldowns, retry counts and queues, when a process takes over and when it stops leading. TestTriggers_ResetForgetsLeaderState pins it.
status: addressed
---
