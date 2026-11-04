---
id: RR-GPNEL8V
type: review-response
title: current falls back to another type than the default
finding: 'RlChannelComposer.vue current: the fallback was pickable[0], a transition type, so Cancel decision emitted select for that type before the selection watch set the default.'
severity: minor
resolution: After the first selection the fallback is the defaultType entry of pickable; before it, pickable[0] stays so the mount still reports the default with a select event (SelectEmits). Story CancelSelectsDefault.
status: addressed
---
