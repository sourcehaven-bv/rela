---
id: RR-47S4ZM
type: review-response
title: The 30s slack could be stored in the hint instead of a second wake function
finding: 'Plan lines 610-617 add wakeAt next to nextDue and change fold, write, noteDue and Resumption.Note to use it, while More still reads the exact hint. A simpler shape: for reminders, store next_due_at as slot + ReminderSlack, and keep the fire condition at slot <= now. The listing window of now + TimerCeiling still includes the channel early, so rounds still coalesce dense slots. recompute, fold, noteDue and Resumption need no change, and there is one function for the due time. The wire due_at still comes from the slot. Plan change: adopt this, or record why two derived times are needed.'
severity: minor
resolution: 'Addressed: next_due_at holds slot plus ReminderSlack, and the fire condition reads the slot. wakeAt is dropped, so there is one due function. The wire due_at stays the slot.'
status: addressed
---
