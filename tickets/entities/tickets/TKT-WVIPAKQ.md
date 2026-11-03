---
id: TKT-WVIPAKQ
type: ticket
title: 'MCP list_inbox: include the waiting message?'
kind: enhancement
priority: low
effort: s
status: backlog
---

## Description

TKT-V05706X adds the waiting message (the message that moved the session into
its current state) to inbox entries on the HTTP inbox, for the web app. MCP
`list_inbox` and `rela channel inbox` leave it out.

Decide whether agents should get it in `list_inbox` too. For: an agent sees what
it is asked without a second `read_session` or channel read. Against: it puts
untrusted message content into every inbox answer and every `watch` wake, which
widens the prompt-injection surface and the answer size for agents.

If yes: an opt-in argument (for example `include: waiting_message`) on
`list_inbox`, the same `InboxRequest.Messages` flag, docs in the MCP guide.

Split off from TKT-V05706X.
