---
id: TKT-VO0KLLZ
type: ticket
title: Browser notifications for channel inbox entries (mentions, replies, waiting sessions)
kind: enhancement
priority: medium
effort: m
status: ready
---

## Description

Channels have an inbox (mentions, replies, waiting sessions), unread counts and
a live sidebar badge (TKT-9YRRM5, RES-B0MJAZ). A user only learns about a new
inbox entry while the rela tab is in front of them. A mention or a session
waiting on the user goes unnoticed while they work in another tab or app.

Consider notifying the user outside the visible page:

- **Browser notifications** (Notification API) from an open but hidden
tab, driven by the existing live inbox subscription.
- **Web Push** (Push API, service worker, VAPID) so a notification
arrives with no rela tab open. This needs stored push subscriptions per
principal and a send job, so it is a larger step.
- **Cheap signals**: unread count in the tab title and favicon.
- **Desktop app**: the shell already shows native notifications from
`desktop.yaml` entity rules (`internal/desktopnotify`); channel inbox entries
could reach it too.
- Per-user preferences: which inbox kinds notify, mute per channel.
Quiet message types (`unread: false`) never notify.

## Constraints

- Entity content and entity existence are secret. A notification shows
only what the inbox GET would show the same principal now. A Web Push payload
passes through a third-party push service, so it carries no content; a click
opens the inbox, which re-gates.
- RES-B0MJAZ (g2) rejected a push stream of entries as a second delivery
path with its own ACL story. A design must reuse the gated inbox read.
- External side effects go on `jobs.Queue` (`.claude/rules/jobs.md`).
- Per-principal state uses `state.KV` or tenant tables, not local files.

## Scope

Planning decides which of the options above form v1 and which become follow-up
tickets.
