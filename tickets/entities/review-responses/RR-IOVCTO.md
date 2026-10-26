---
id: RR-IOVCTO
type: review-response
title: Due learns of the reminder post only through a closure flag
finding: 'Plan lines 470-475 and 592-599: Due must return the advanced record only when the wrapped Remind posted. That information can reach Due only through closure state shared between the two callbacks, on top of the existing Decide-to-Due capture in withSessionDue (timers.go:77-85). ScheduleFunc''s contract says a backend may call it more than once if it retries (internal/channels/session.go:41-45), so a flag left over from an earlier attempt goes stale. Plan change: keep the optional callback, which avoids a new Store method, but return the effect explicitly. Remind returns ReminderPost{Type, Props, TriggeredBy, Name, Record}. The store validates the record (name, cap, normalization), writes c.Session.Reminders[Name] = Record together with the message, and only then calls Due. Due stays a pure function of the committed channel, and the store conformance test can assert the stored record directly.'
severity: minor
resolution: 'Addressed: Remind returns ReminderPost with Name and Record. The store validates, normalizes and stores the record with the message before calling Due, so Due is a pure function of the committed channel. Conformance asserts it.'
status: addressed
---
