---
id: RR-TI09ZA
type: review-response
title: The composer briefly shows the previous state's transitions
finding: 'follow() calls setChannel(page.channel) before refetchHead (frontend/src/composables/useChannel.ts:333-343), so between the frame and the head answer the stepper shows the new state while `_actions.transitions` still lists the old one. The composer then shows "moves the session to Contained" in Contained. Plan change: build composerTransitions from `_actions.transitions` ∩ `head.channel.session.transitions[].message_type` ∩ post. That list refreshes with every frame. Take `to` from the head session transition and `guarded` from the descriptor. Add a channelView.test.ts row for a head whose session moved ahead of its actions. Also say in the plan that sessionMoved is computed before setChannel, as stateMoved is.'
severity: minor
resolution: 'Plan updated: composerTransitions = _actions.transitions ∩ head session transitions ∩ post, to/guarded from the head (Approach 6); sessionMoved computed before setChannel (Approach 10); AC 15 moved-ahead row.'
status: addressed
---
