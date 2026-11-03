---
id: RR-D8NY8I
type: review-response
title: Say how presets interact with quiet types (TKT-03DMXH)
finding: 'The plan never mentions display: collapsed or hidden. RlChannel keeps the decisionSeq row regardless of the fold (frontend/packages/rela-components/src/components/channel/RlChannel.vue:206), so row presets survive folding. The preset message itself may then be hidden in the log. State this and add a ChannelsPanel case where the decision type is display: hidden and the preset row still shows.'
severity: minor
resolution: AC 11 states that presets follow the decision row that RlChannel keeps when quiet types fold, and that the preset message may be hidden. ChannelsPanel gains a "hidden decision type keeps its presets" case.
status: addressed
---
