---
id: RR-G2YHBV
type: review-response
title: '"Waiting on me" for unstored or unset sessions is something the inbox cannot reproduce without scanning every anchor'
finding: 'Plan edge case (lines 567-568) makes `waiting_on_me` true on a cardinality-one channel that is not stored, and that channel has no row at all. A stored channel that never moved stores an empty session state (`channels/session.go:19,96`; `sqlchannels session_state`), and that reads as whatever `initial:` the current config names. Approach 9 (lines 430-444) calls status "a pure function of (type, stored state)" and plans `ListBySessionState(type, states)`. That index finds neither case, and a reload that changes `initial:` changes what the empty state means. Change: make `waiting_on_me` false for an unstored channel, to match the stored-and-open rule for `due_at`, and say the inbox lists only stored channels. In Approach 9, state that a state query must map `''''` to the configured initial state. Fix the "pure function" wording.'
severity: significant
resolution: The unset stored state means initial everywhere (AC 13); waiting_on_me is false for unstored channels; the inbox lists stored channels and maps '' to initial.
status: addressed
---
