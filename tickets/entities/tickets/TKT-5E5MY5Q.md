---
id: TKT-5E5MY5Q
type: ticket
title: 'Threads 4: thread side panel in the web app'
kind: enhancement
priority: medium
effort: l
status: backlog
---

## Description

Slice 4 of RES-TU7ZMMR (from TKT-UQ0BM5G). The web app shows threads in a side
panel instead of inline.

### Scope

- The main stream reads `top=1`. A parent with replies shows "N replies" and the last reply time.
- Clicking it opens the thread in a side panel (a sheet on mobile) with its own composer, paging and live updates (`thread=<seq>&after_rev=`).
- Inline exceptions (choice answers, replies that moved the session) stay in the stream, with their quote as today.
- Follow and mute in the panel; unread badge per followed thread; the inbox `thread` entry opens the panel.
- The panel marks the thread read as the stream marks the channel read.
- The removed parent shows as a stub; its thread stays readable and the composer is off.
- Vitest units and an e2e test for post, reply, panel, unread and follow.

### Acceptance criteria

- A reply posted in the panel does not appear in the main stream, except as an inline exception.
- Choices and session decisions behave as before.
- Opening the channel leaves followed threads unread until their panel is opened.
