---
id: RR-KNXYNQ
type: review-response
title: A dirty shard skips its retry delay whenever a round reads it
finding: 'internal/realtime/trigger.go round.shard and round.save. round.shard loads a dirty shard with changed=true, and the first loop of save writes every changed shard without looking at d.next. Only dirty shards that no round touched wait for the doubling delay. During a KV outage, every round that has work for that rule (one every 500 ms under load) writes the shard again. The resolution''s claim that ''a failing shard save now waits with the same doubling delay'' holds only for idle shards. Fix: in the first loop, skip a key whose dirty entry is not yet due, unless the round changed it beyond what dirty holds. Or only send to save the shards whose dirty next has passed, and keep the others in dirty with the merged data.'
severity: minor
resolution: Save now skips a changed shard whose unsaved entry is not yet due and folds the round's data into that entry instead. TestTriggers_FailingSaveDoesNotRepost checks that a round reading the shard makes no write before the delay.
status: addressed
---
