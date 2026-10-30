---
id: RR-OQOR5K
type: review-response
title: message_types.<type>.button reads as a button on the message, or on every button of the type
finding: 'On a message type, every existing button concept is a button drawn on a message of that type: actions are "the buttons on a message" (internal/realtimecfg/compiled.go:460) and choices draws reply buttons on the question (compiled.go:466-473). A reader of realtime.yaml will take message_types.allow.button either as a button on each Allow message, or as the look of every button that posts allow: preset buttons, choice-answer buttons, the type button. Under the plan it is none of those; it is only the type decision button, and Q3 says presets do not inherit it. The symmetry argument with on.<type>.button (Approach 2) holds for the key name only: one is a full button (label, props) and the other is two style options. Change the plan: name the key decision_button (it styles the type''s decision buttons; the rejected decision: name collides with a property, decision_button does not), and say in the docs that it does not affect actions, choice buttons or the composer. If the key stays button:, Q3 must change to inheritance so the name means what it looks like.'
severity: significant
resolution: Renamed the key to message_types.<type>.decision_button throughout (Go DecisionButton, wire decision_button, ChannelDecisionButtonSchema). The docs row says it does not affect actions, choice buttons or the composer.
status: addressed
---
