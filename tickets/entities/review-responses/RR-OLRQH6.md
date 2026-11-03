---
id: RR-OLRQH6
type: review-response
title: Each message costs three requests per viewer
finding: '§3 and the live topics: a channel change moves both the head topic and the anchor''s list topic (channelKeyTopics), so every viewer with the panel open refetches the list, the head and an after_rev page per message. Fix: while a channel is open, refetch the list only when the frame is for the list key and debounce it, or drop the list subscription while one channel is open. The after_rev page carries `channel`, so the head refetch is only needed for ephemeral messages and `_actions`. Pin the request count in useChannel.test.ts.'
severity: minor
resolution: The list subscription is dropped while a channel is open, the head is refetched only when needed, and the request count per frame is pinned in a test (R18, AC 19).
status: addressed
---
