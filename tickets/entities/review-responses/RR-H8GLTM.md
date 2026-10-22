---
id: RR-H8GLTM
type: review-response
title: Typing can evict the user's own operator signal, and an operator post can evict typing
finding: 'Step 6 replaces the identity''s typing signal first, but when none exists the generic own-oldest rule still runs (internal/realtime/ephemeral.go:96-99). A keystroke by an identity at EphemeralRingShare (8), at the ring size, or at MaxEphemeralPerIdentity (32) then silently overwrites its oldest operator signal, for example a status signal. In the other direction, an operator post can overwrite the typing signal. Typing also takes one of only 4 rings per identity (MaxEphemeralChannelsPerIdentity, internal/realtimelimits/limits.go:49-53), so a user with operator signals in 4 channels gets a silent 429 for typing in a fifth. Plan change: a typing signal only ever replaces the identity''s own typing signal and otherwise faces the caps (429). The generic rule skips typing signals when it picks a victim. Add both rows to TestEphemeral_Caps and document the ring-count interaction in the guide.'
severity: minor
resolution: 'Plan updated: typing uses its own per-channel map keyed by identity, outside the ring, with refuse-only caps MaxTypingPerChannel and MaxTypingChannels; TestTyping_SeparateFromRing and TestTyping_Caps added.'
status: addressed
---
