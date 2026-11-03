---
id: RR-G8DBHS
type: review-response
title: isReminder in the frontend has no consumer
finding: 'Plan lines 661-663 add REMINDER_MARKER and isReminder to frontend/src/types/channels.ts "so slice 2''s message actions code can use it later". Nothing uses them in this slice, and the server already withholds _actions. That is speculative dead code. Plan change: add them in the slice that renders reminders, and keep only the wire-field mirror here.'
severity: nit
resolution: 'Addressed: the frontend isReminder and REMINDER_MARKER are dropped. This slice mirrors only the wire fields; the slice that renders reminders adds the helper.'
status: addressed
---
