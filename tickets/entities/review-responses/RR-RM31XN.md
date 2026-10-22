---
id: RR-RM31XN
type: review-response
title: session.entered_at is zero in guards for a channel that is not stored yet
finding: 'sessionNow uses the zero time for a cardinality-one channel that does not exist yet. The append then creates the channel with created_at = the post time, and reads show entered_at = the post time. So a guard on session.entered_at (days_between, date_add) sees year 1, while a reader sees today. Use c.now for a channel that is not stored yet. Also note a slice-3 seam: an unset session on a channel that existed before its type gained a session reports entered_at = the channel''s created_at, possibly months ago, so slice-3 `after:` timers on the initial state would fire at once for old channels. Decide this semantics now, because the wire shows it from this slice on.'
severity: minor
resolution: 'Plan updated: entered_at is the move time, created_at for an unset stored channel, and the operation clock for an unstored channel, in guards and reads alike (TestSessionEnteredAt).'
status: addressed
---
