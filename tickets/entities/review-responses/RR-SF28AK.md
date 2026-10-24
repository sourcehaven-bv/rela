---
id: RR-SF28AK
type: review-response
title: Config key session.timers names the opposite thing from the wire field timers
finding: 'In the plan, config session.timers (lines 46 and 103) maps to wire reminders, to Go SessionReminder and to the guard session.reminders. Wire SessionView.Timers (internal/realtime/session.go:42-45), Session.HasTimers() and SessionTimer all mean the per-state after: entries. A reader of realtime.yaml sees timers:, while the HTTP answer calls the same entries reminders and uses timers for something else. The plan also needs a refusal of session.states.<s>.timers (line 412) only because the name is ambiguous. Operator-facing keys are expensive to rename once released, and the research''s timers: (res-sessions-final.md:140) was a sketch. Plan change: name the key session.reminders. Config, Go, guard and wire then use one word, and timers keeps meaning after:. Change the refused key to session.states.<s>.reminders ("reminders are declared once, under session.reminders, with in:"). Update the example, the docs and AC 1-3.'
severity: significant
resolution: 'Addressed: the config key is session.reminders. The refused key is now session.states.<s>.reminders, and session.timers is refused with a pointer to reminders and after:. Example, docs and AC 1-3 updated.'
status: addressed
---
