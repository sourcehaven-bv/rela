---
id: TKT-O6RC8Z
type: ticket
title: Server-known quiet message types (unread, inbox)
kind: enhancement
priority: low
effort: s
status: backlog
---

## Description

TKT-03DMXH adds `display: collapsed | hidden` per message type in
data-entry.yaml. It is SPA-only, so the unread badge still counts quiet
messages, and an inbox mention or reply row opens a channel where the message is
hidden. Add a key in realtime.yaml (not a server read of data-entry.yaml) so the
server can leave quiet types out of unread counts, and let inbox rows reveal the
message they point to.
