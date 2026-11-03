---
id: RR-TGEM02
type: review-response
title: Read window LastAt differs between the file and SQL backends
finding: 'internal/channels/filechannels/reads.go:184,336 versus internal/channels/sqlchannels/sql.go:546. (1) The file backend takes LastAt from the head of the last message record. mutateRemove (filechannels/ops.go:313) rewrites a removed message without At, so when a channel''s last message is removed, its LastAt becomes the zero time and the channel sorts to the bottom of its group in SortReadWindow. On the file backend that channel is the first one cut at MaxReadWindow, and the last one listed in unread_channels. SQL keeps last_at at the post time. (2) A channel with no messages that is closed, archived or reopened gets LifecycleAt in the file backend, but keeps created_at in SQL, because insertChannel sets last_at once and no lifecycle write updates it. The plan says the backends agree on the window, but testListReads/testListReadsWindow cover neither case. Fix: in logHead, skip records with a zero At (or keep walking back to the newest record that has a time), and use max(lastMessageAt, LifecycleAt(c)) on the file side. Alternatively, define LastAt as message-only on both backends and drop LifecycleAt for channels that have messages. Add a conformance row for "last message removed" and one for "closed channel with no messages".'
severity: minor
resolution: Both backends use LastAt = max(newest stored message time, LifecycleAt); file store keeps last_at in meta.yaml; conformance row ListReadsLastAt covers removal and closed channels
status: addressed
---
