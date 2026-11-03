---
id: RR-0NX10P
type: review-response
title: File-tier edit brings back rows that the cap step evicted or skipped, so file and SQL backends diverge
finding: 'internal/channels/filechannels/ops.go:333-347 with addressed.go:153-171 and :254-264. Every file-tier Edit rebuilds the message''s rows. It sets healed[target]=true for each kept target and rewrites any missing mention row at msg.At. editReplyTarget also re-adds a missing reply row. The rebuild cannot tell a row that a failed write lost from a row that channels.Evict removed on purpose, or one that LogSkippedRow skipped. Scenario: B holds 200 unread rows from A in one channel. A posts again, and A''s oldest unread row (seq s0) is evicted, as AC 7 requires. A then fixes a typo in message s0 and keeps the `@B` span. On the file tier, s0 is written again as unread at its old time and evicts A''s next-oldest unread row, so B''s inbox shows s0 again. On sqlchannels, editAddressed writes only AddedMentions, so s0 stays gone. A row skipped because the set was full of other posters'' unread rows also comes back on any edit on the file tier, but never on SQL. channelstest has no case for edit-after-evict, so the conformance suite cannot catch this. Fix: record evicted or skipped rows (for example a tombstone set in mentions.json), or limit healing to rows lost by a failed post-log write.'
severity: minor
resolution: File-tier edit now writes rows only for targets the edit adds, like the SQL tiers; repair-on-edit removed. Conformance case in testAddressedTargetCap plus TestAddressed_EditWritesOnlyAdded.
status: addressed
---
