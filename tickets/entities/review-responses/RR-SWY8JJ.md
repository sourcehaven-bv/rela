---
id: RR-SWY8JJ
type: review-response
title: Reminder posts are a third write path the plan misses
finding: 'The plan says two sites build a typed message: service.go:450 and system.go:62 "for triggers and reminders". system.go post is called only from trigger.go:1282. Session reminders never reach Append: realtime/reminders.go:246 builds a channels.ReminderPost, and the store writes it through channels.ReminderMessage (channels/session.go:238-249), called from filechannels/ops.go:1385 and sqlchannels/sql.go:1247. With the plan as written, a reminder of a type with unread: false is stored without the flag and counts. TestSystemPost_NoUnreadType ("a reminder of a non-counting type") would test the trigger path and pass anyway. Fix: add NoUnread to ReminderPost, copy it in ReminderMessage, and set it in reminders.go from the compiled post type. Add a channelstest reminder case on all four backends and add reminders.go and session.go to Files to modify.'
severity: critical
resolution: ADDRESSED ReminderPost gains NoUnread, ReminderMessage copies it, reminderStep sets it from the post type; new TestReminderStep_NoUnreadType, TestReminderMessage_CopiesNoUnread and channelstest testReminderNoUnread on four backends; trigger test renamed TestTriggerPost_NoUnreadType; session.go and reminders.go added to files (AC 2, AC 3, Approach 2 and 3).
status: addressed
---
