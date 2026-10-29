---
id: RR-2MCLOZ
type: review-response
title: display on a signal type is undefined
finding: 'Signals are message types with persist: ephemeral, rendered by ephemeralText (RlChannel.vue:178-202, 403-408). AC 8 hides quiet signals, but the plan does not say what collapsed means for a signal, and validation does not distinguish signal types (ChannelSchema has no persist fact, channels.go:46-55). Change the plan: state that both values keep a signal out of the signal area while the toggle is off, and document it, or refuse collapsed on a signal type and add the fact to ChannelSchema.'
severity: minor
resolution: Both values now keep a signal out of the signal area while the toggle is off, with no new ChannelSchema fact (AC 8, step 6). The docs state this.
status: addressed
---
