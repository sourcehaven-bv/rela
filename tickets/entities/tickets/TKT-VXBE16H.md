---
id: TKT-VXBE16H
type: ticket
title: Place a channel type above the relations by configuration
kind: enhancement
priority: high
effort: m
started: "2026-11-03"
status: planning
---

## Description

On an entity page the channels panel is the last thing on the page. It sits
below every view section (relations included), the documents panel, the
"Mentioned by" panel and the comments (`EntityDetail.vue`, `ChannelsPanel`). On
a ticket of the agent runner PoC that is about 1200 px down, so a session that
waits on the reader is hard to find (FINDINGS.md, UX finding 1, gap 4).

No configuration can move it. `data-entry.yaml` presents channel types in its
`channels:` block (message `fields`/`display`, `session.display` from
TKT-56TPOTE) but has no placement key, and `pages:` builds separate tabbed
screens; it does not lay out the entity page.

Add `channels.<type>.placement: above_relations`, which places the channel type
directly before the first view section that does not come from the entry (in the
generated view: after Properties and the body, before the relations). Without
the setting the page stays as it is today. A tab placement is TKT-B75I7LH.

Plan: `.ignored/plan-TKT-VXBE16H.md` (approved 2026-11-03, tab moved out).

## Acceptance

- `placement: above_relations` puts the channel type before the first
relation section, on the full page and in embedded detail views.
- Without the setting the channels panel stays at the bottom of the page.
- Any other value, `tab` included, is a load error that names the channel
type.
