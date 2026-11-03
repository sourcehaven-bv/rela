---
id: RR-YO45BU
type: review-response
title: Stale records plus the 8-record cap can block a channel in backoff
finding: 'Plan lines 576-578 keep the records of removed or renamed reminders until the next move. Lines 482-484 cap stored records at 8, and a non-nil schedule replaces the whole map (internal/channels/session.go:61-63). Suppose a reload renames all 8 reminders. The reminder step''s Due then returns 8 stale plus 1 new record. ApplySchedule refuses that with ErrInvalidMessage, the write fails, holdBack applies, and the channel''s reminders stop until a person moves the session. Plan change: every schedule that returns a non-nil map (the reminder step, stepSchedule and resumeSession) keeps only records of reminders declared under the current config and drops the rest. Inactive records then need no special rule. Add a test that renames 8 reminders on a session with 8 stored records.'
severity: minor
resolution: 'Addressed: every schedule that returns a non-nil map keeps only records of declared reminders. The cap applies to the final map. TestSchedulesPruneUndeclared renames 8 reminders on a session with 8 records.'
status: addressed
---
