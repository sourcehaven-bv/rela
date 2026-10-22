---
id: RR-9ABKR3
type: review-response
title: Every frame rebuilds the session view and composer transitions and runs both focus watchers
finding: 'useChannel.setChannel replaces head.value on each frame, so in OpenChannel.vue:140-145 `session` and `transitions` return new objects each time. Through transitionByType and offered, the composer''s two `offered` watchers (RlChannelComposer.vue:181-201) run and query the DOM on every message in a busy channel, and the stepper re-renders even when the session did not change. The cost is small, but it is avoidable. Fix: key the computeds on session.generation, session.state, _actions.transitions and post (or compare shallowly before replacing), so they keep the same array when nothing changed.'
severity: nit
resolution: OpenChannel reuses the session and transitions views when descriptor, generation, state, channel state and actions are unchanged; a panel test pins object identity.
status: addressed
---
