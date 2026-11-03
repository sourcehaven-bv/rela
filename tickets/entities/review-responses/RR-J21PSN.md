---
id: RR-J21PSN
type: review-response
title: CLI inbox prints nothing about unread when there is none next to the waiting section
finding: 'internal/cli/channel_inbox.go:90. With the default kinds, "No session waits on you." is printed and the unread section is dropped. The output then looks the same as a cursor page, where unread is not computed at all. UnreadCount != nil already tells the two apart. Fix: print "No unread channels." whenever UnreadCount is non-nil and zero.'
severity: nit
resolution: CLI prints No unread channels when the count is zero and omits the section on cursor pages; tests and CLI reference updated
status: addressed
---
