---
id: RR-NJB38V
type: review-response
title: A damaged reminder record with a negative count is not treated as a damaged row and blocks later writes to the channel
finding: 'internal/channels/sqlchannels/sql.go:511 (decodeReminders) and internal/channels/filechannels/file.go reminders() accept any name and any count from storage. checkReminder only runs at write time, in internal/channels/session.go:313 and :339. Example: a tampered or corrupted session_reminders value such as {"status-update":{"since":1,"count":-1}} scans cleanly, so scanDue does not skip the row with its damaged-row warning. pruneReminders (internal/realtime/reminders.go) keeps the record because the name is declared. Every SetSession, every session post (stepSchedule, internal/realtime/timers.go:97) and every SetLifecycle (sql.go:785, through resumeReminders) then fails in ApplySchedule with ErrInvalidMessage. The channel becomes unpostable and cannot be closed or reopened, and the timer loop retries it every backoff. This fails closed and requires storage write access, so it is not exploitable from the API. Fix: in decodeReminders and meta.reminders(), run channels.checkReminder (export it or add a channels.ValidReminder) and return a damagedRowError on failure, so the due listing skips the row with a warning as it does for damaged restarts. Alternatively, drop invalid records on read.'
severity: minor
resolution: 'channels.CheckReminders validates records on read in sql and file stores; a bad record is a damaged row that the due listing skips with a warning. Tests: TestListSessionsDue_SkipsDamagedRow (sqlite, pg), TestListSessionsDue_DamagedReminderRecord (file).'
status: addressed
---
