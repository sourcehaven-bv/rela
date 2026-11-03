---
id: RR-FOHOY5
type: review-response
title: Dirty shards of removed rules are written back after collect deleted them
finding: 'internal/realtime/trigger.go:598-623 and 932-936. collect() removes a rule from the index and deletes its shards. save() then iterates every key in t.dirty, including shards of the removed rule, and Puts them again. That leaves unreferenced KV keys that nothing will ever GC. Fix: in collect, delete t.dirty entries whose key has the prefix of a removed rule id, and skip dirty keys not referenced by the current index in save().'
severity: minor
resolution: Collect and baseline call forgetRule, which drops the queue and unsaved shards of the rule. Save also discards an unsaved shard whose rule or shard layout is no longer in the index.
status: addressed
---
