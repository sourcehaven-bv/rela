---
id: RR-66G6FK
type: review-response
title: The inbox applies the kinds filter after the 500-row store window, so kinds=reply can miss unread replies
finding: 'internal/realtime/inbox.go:681-691. addressedWindow calls ListAddressed(reader, MaxAddressedWindow) for both kinds and drops rows of unasked kinds afterwards. The rows of undeclared channel types are also dropped after the cut. The SQL inner scan (sqlchannels/addressed.go:24-26) and the file walk also take the 2000 newest rows regardless of kind. Scenario: a user has 500 unread mention rows newer than one unread reply. GET /api/v1/_inbox?kinds=reply answers mentions=[] and mention_count=0, although an unread reply exists. A list_inbox watch with kinds=["reply"] wakes and then sees nothing. AC 10 says kinds=reply lists replies, and the docs describe the window only as "2000 newest rows, 500 unread". Fix: pass the asked kinds to ListAddressed (WHERE kind IN (...) in the inner scan, plus a filter in the file walk), or document that the window is shared by both kinds.'
severity: minor
resolution: ListAddressed takes a kind and all tiers filter inside the scan, with channel_mentions_kind_idx. Conformance ListAddressedKind/ListAddressedKindScan and TestInbox_ReplyKindPastMentions.
status: addressed
---
