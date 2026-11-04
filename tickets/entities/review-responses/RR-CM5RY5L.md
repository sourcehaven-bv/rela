---
id: RR-CM5RY5L
type: review-response
title: Above-relations block remounts when the sections change
finding: 'EntityDetail rendered the above_relations ChannelsPanel inside the keyed fragment of the section at the insert index, plus a copy after the loop. A reload on the same entity that adds or drops a section before the boundary (hide_empty, a config refresh) moved the block into another fragment, so it unmounted and remounted: the open channel, the selection and composer text were lost and both live keys re-subscribed.'
severity: significant
resolution: The block is now an item of the keyed section list (withAboveRelationsBlock in channelPlacement.ts) with a fixed key that cannot collide with a section slug, so Vue moves it instead of remounting. EntityDetail.channelPlacement.test.ts reloads with a new section before the boundary and asserts no remount; a mutation that keys the block by index fails it.
status: addressed
---
