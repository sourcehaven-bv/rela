---
id: RR-TAXYJ4
type: review-response
title: Corrupt index or shard recovery leaves the old shard keys in the KV for good
finding: 'internal/realtime/trigger.go round.run (corrupt index) and round.reset (corrupt shard). On a corrupt index every rule baselines again into shardsFor(n) keys, and on a corrupt shard the rule''s index entry is deleted. Neither deletes the old keys. The new baseline overwrites them only when the shard count happens to match. With a corrupt index, the seen-sets of rules that were removed meanwhile can never be collected, because collect works from the index. Each corruption therefore leaves orphan keys behind, possibly including the corrupt value itself. Fix: in reset, call dropShards(ru.id, meta.Shards) with a best-effort warning. For a corrupt index, list the realtime/seen/ prefix if the KV supports it, or document the leak.'
severity: minor
resolution: Reset now deletes the corrupt rule's shards. After a corrupt index, the shards of every current rule are deleted under every possible shard count before the baseline; shards of rules removed in the meantime cannot be found because the KV cannot list keys, and a code comment says so.
status: addressed
---
