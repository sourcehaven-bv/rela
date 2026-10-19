---
id: RR-BEX7AE
type: review-response
title: At-least-once has no failure policy, so a broken step reposts forever
finding: 'D6 and §3 step 8 post first and save after, but the plan never says what a failed `systemPost`, `KV.Put` or `Rows` does. If the shard save keeps failing (KV error, oversized shard), every round and every reconcile posts the same messages again. If a post keeps failing (a literal longer than the property''s `max:`, which `triggerProps` in realtimecfg/validate.go does not check, or the 200-channel cap), it fails forever. Define the outcome per failure: log, then mark as seen, or back off per rule. Check `max:` and MessageBytes for trigger literals at load. Add an AC with a KV that always fails.'
severity: significant
resolution: 'Per D6: a failing post or seen save is retried for 3 rounds, then logged and marked seen, so it never loops. Trigger literals are checked against max and MessageBytes at load. An AC with an always-failing KV pins the bound.'
status: addressed
---
