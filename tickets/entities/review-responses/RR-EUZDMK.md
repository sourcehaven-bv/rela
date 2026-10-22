---
id: RR-EUZDMK
type: review-response
title: Two sources for the current state's transitions
finding: 'The head already carries session.transitions (message_type, to, by) for the current state (internal/realtime/session.go:85-91, internal/dataentry/channels_wire.go:37-50). The plan adds per-state transitions to the descriptor, and the only new fact they bring is `guarded`. After a realtime.yaml reload the head and a stale descriptor can disagree, and the plan uses the head for `next` (AC 14) while composerTransitions(ct, head) reads `to` and `guarded` from somewhere unstated. Plan change, the simplest one: add `guarded` (omitempty) to SessionTransitionView, v1.ChannelSessionTransition, the TS type and CHANNEL_WIRE_FIELDS. Drop transitions from ChannelSessionStateSchema; the descriptor keeps label, initial, and the states with label and final. Otherwise, name for each UI piece which source it reads.'
severity: minor
resolution: 'Plan updated: descriptor keeps label, initial, ordered states with label and final, no transitions; guarded added to SessionTransitionView, v1.ChannelSessionTransition, TS type and CHANNEL_WIRE_FIELDS; source table in Approach 2.'
status: addressed
---
