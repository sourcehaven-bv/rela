---
id: TKT-EONZTMZ
type: ticket
title: 'Direct messages 4/4: DMs in the web app'
kind: enhancement
priority: medium
effort: m
status: backlog
description: Message button on a person's page, a DM list with unread counts and a DM view in the SPA (DEC-42MOZY2).
---

## Description

Show direct messages in the Vue SPA. Design: DEC-42MOZY2.

- A "Message" button on a person's user entity page opens the DM.
- A DM list (sidebar entry or inbox section) with unread counts.
- A DM view that reuses the channel message list and composer.
- Live updates and the typing indicator work as in other channels.

## Acceptance criteria

1. From a person's page, "Message" opens the DM and a posted message
appears for the other person live.
2. The DM list shows unread counts and clears them on read.
3. The button is absent on your own page and when DMs are off.
4. E2E test covers open, post, receive and read.
