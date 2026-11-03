---
id: RR-OBQ9QW
type: review-response
title: Exactly-once creates on the database tiers depend on txChannelsFor joining
finding: 'txChannelsFor returns nil when the channel store is filechannels or is not a view of the same database (appbuild/realtime_promote_sqlite.go:21-25, _postgres.go:21-24, _nodb.go:14). In that case InTx cannot carry the progress write, and the file-tier rule must apply. The plan states the semantics per tier (sqlite or postgres versus file) rather than per join. Plan change: decide by whether txChannelsFor joins. Without a join, use the `unconfirmed` rule. Name this in decision 6 and in AC 12.'
severity: minor
resolution: 'Semantics now depend on whether txChannelsFor joins: exactly once with a join, the unconfirmed rule without one. Stated in decision 6, AC 11 and AC 12 (TestOutputs_NoJoinUsesUnconfirmed).'
status: addressed
---
