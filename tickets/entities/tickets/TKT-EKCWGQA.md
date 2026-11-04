---
id: TKT-EKCWGQA
type: ticket
title: Channels entry in the entity page jump bar
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Description

The entity page's jump bar lists the view sections with a heading. It does not
list the channels block, so a reader who opens an entity from a board or list
has no quick way to reach a channel that waits on them, unless the operator sets
`channels.<type>.placement` (TKT-VXBE16H).

Decide whether the jump bar should get a "Channels" entry (per block, with the
count of sessions waiting on the reader) for every placement, including the
default bottom one. That changes the default entity page for every project with
channels, so it needs a design decision and is kept out of TKT-VXBE16H, which
leaves the default page unchanged.

## Acceptance

- Decision recorded: entry for all placements, only for placed blocks, or
none.
- If built: the entry scrolls to the block and shows the waiting count.
