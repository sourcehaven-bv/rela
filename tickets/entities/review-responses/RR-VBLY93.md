---
id: RR-VBLY93
type: review-response
title: The summary live subscription is not pinned
finding: AC 7 tests the inbox page subscription only. The sidebar badge subscribes to /_inbox/summary, which hashes InboxView.Summary (inbox.go:193-197) and has no last_seq. A non-counting post into a channel already listed as unread moves the page's last_seq and sends a page frame, but should send no summary frame. Add TestLiveInbox_NoUnreadPostSummaryNoFrame, because the badge is the surface this ticket exists to fix.
severity: minor
resolution: ADDRESSED TestLiveInbox_NoUnreadPostSummaryNoFrame added to AC 7 and the test plan.
status: addressed
---
