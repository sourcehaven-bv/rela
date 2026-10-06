---
id: RR-59SCDU
type: review-response
title: Comment verification depends on thread order
finding: verify compared comment lists in backend order; sqlitecomments orders by created_at and id while a thread file may not.
severity: minor
resolution: utcComments sorts by (CreatedAt, ID) before comparing. TestRun_CommentsOutOfOrder.
status: addressed
---
