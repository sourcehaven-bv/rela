---
id: RR-J3L32Q
type: review-response
title: Stale descriptor shows a false "Stranded" warning while the composer still offers transitions
finding: 'frontend/src/components/entity/channels/channelView.ts:263-281 (sessionView): a head state missing from the held descriptor sets stranded=true. RlChannelSession then shows the warning tag and "<state> is no longer a state of this session. Messages do not move it." That text is false, because the state is new and the server will move it. composerTransitions (channelView.ts:290-308) checks only cs.stranded, so in the same frame the composer marks types as moving the session and shows "Sending this moves the session to <name>". During the descriptor-reload window the UI contradicts itself, and every reader sees a warning. Fix: distinguish unknown-to-descriptor from server-stranded. Keep `stranded` only for cs.stranded. For an unknown state, render the current label by name with no warning (for example a neutral "current: <name>" item, or hide the stepper until checkNames'' reload lands).'
severity: minor
resolution: A state the held descriptor lacks shows a neutral Loading step with no warning and no composer marks; Stranded only from the server or after a reload that still lacks it.
status: addressed
---
