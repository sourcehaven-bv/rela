---
id: RR-FCUUFV
type: review-response
title: Not moving on a gate mismatch is correct and holds on every backend; document why refusal is not possible
finding: 'I checked that every backend runs the decider through channels.DecideSession (internal/channels/session.go:387-402, called from filechannels/ops.go and sqlchannels/sql.go). So Admit-then-Decide under the lock is uniform, and a re-run (the comments say a backend may run the decider more than once) resets `failed` and the mark. A gate refusal cannot become a 4xx without giving Decide an error result across the four stores, which the plan puts out of scope. Not moving (and not falling through) is consistent with outputGate today (outputs.go:269-279, 304-336). With expect_generation, state, generation and entered_at cannot change between the pre-check and the lock, so only `session.reminders` counters can switch the entry. AC 9''s test should drive exactly that (a reminder-count guard), not a generation change, which Admit would already refuse for a person''s post with expect_generation. Note in the plan that the gate only constrains entries with outputs: a switch to an entry without outputs under the lock still moves, which is acceptable but not "only the entry the pre-check checked".'
severity: minor
resolution: Rewrote AC 9 to switch entries through a reminder-count guard, and documented that the gate only holds moves along entries with outputs and why a gate refusal cannot become a 4xx.
status: addressed
---
