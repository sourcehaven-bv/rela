---
id: RR-1S5JNU
type: review-response
title: A handoff or claim re-entry lets any holder push back deadlines and reset escalation counters indefinitely
finding: 'A re-entry sets a new `entered_at` and clears the reminder records (`channels/session.go:29-32`, "A move clears it"). So a re-entry restarts `since: state` timers, restarts reminder spans and their `max`, and resets `session.reminders[...]` to 0. The testdata already escalates on that counter (`valid.yaml:54`, `escalate-stalled when session.reminders[''status-update''] >= 3`). The claim transition has no `roles:`. Any poster of `claim` (lead or dpo) can therefore re-enter at will. That includes a no-op handoff to the current holder (plan edge case, line 563), and the escalation never fires. Guards cannot read the entity (`session.go:578-580`), so the operator cannot block a self-handoff. Documenting `since: session` (Approach 5) does not cover reminder counters. Change: (a) add a load warning, or a refusal, for a person-fireable re-entry in a state that has `since: state` timers, active reminders, or a guard that reads reminder counts. Name the deadline reset in the message. (b) In the docs and the example, model a handoff as the research''s use case 4 shape (`res-sessions-final.md:89`, "acknowledge step with timeout"): move to a `handed-off` state that waits for the new holder, with its own timer. (c) Make decision 3 "refuse or warn now; a `keep_timers` re-entry is a follow-up", not "confirm the restart".'
severity: significant
resolution: 'Load refuses a re-entry a waiting role can fire in a state with since: state timers or reminders (AC 8); the example and testdata use a handed-over state with since: session.'
status: addressed
---
