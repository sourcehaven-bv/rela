---
id: TKT-B75I7LH
type: ticket
title: Channel placement in its own tab on the entity page
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

Follow-up to TKT-VXBE16H, which adds `channels.<type>.placement:
above_relations`. This ticket adds `placement: tab`: the full entity page gets a
tab bar under the header with a Details tab and one tab per tab-placed channel
type, labelled with the channel type's label and showing the number of sessions
that wait on the reader. Until this ships, `tab` is rejected as an unknown
value.

The design was planned and reviewed under TKT-VXBE16H (see its plan); the parts
that only apply to tabs move here:

- Tab bar and panels in a new `EntityPageTabs.vue` (Details slot);
`RlViewTabs` gains a `label` prop and an optional `badge { text, label }`;
`show-add` false; tabpanel wiring.
- Active tab in `?tab=<type>` on `EntityView` only (`routeTabs`); embedded
hosts get no tabs and put tab-placed types in the bottom block.
- `?channel=` wins over `tab` and selects the tab with one replace before the
panel mounts; a `tab` the new entity's type lacks is removed on entity switch.
Tab ids: `details` and `channel-<type>`.
- A summary list (`useEntityChannels` with a null anchor when no tab type
attaches) gives tab visibility and waiting counts.
- Every block stays mounted once shown (`v-show`). A block not on screen is
inactive: no live list, no live channel (`useChannel` path null), no mark-read
(`useMarkRead` active source), refetch on return.
- Budget with tabs: only active blocks subscribe; full page worst case 8,
embedded detail 6, worst stream 14 of 16; pin with a budget test.

Review findings deferred here from TKT-VXBE16H: RR-SY7QUQQ, RR-RP4N6QL,
RR-T1CKYTL (tab part), RR-YSSDE4P, RR-X4FLZV0, RR-FH331WE, RR-2R9PQQY,
RR-OFREEMB.

## Acceptance

- `placement: tab` gives a channel type its own tab on the full entity page
with a waiting count; an inbox link to a channel in the tab opens the tab.
- A hidden Details or tab block marks nothing read and holds no live
subscription; a composer draft survives a tab switch.
- Embedded hosts show no tab bar.
