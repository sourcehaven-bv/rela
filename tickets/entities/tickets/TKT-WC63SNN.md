---
id: TKT-WC63SNN
type: ticket
title: Let data-entry.yaml mark message types people do not post
kind: enhancement
priority: medium
effort: s
started: "2026-11-10"
completed: "2026-11-11"
status: done
---

## Description

The web app offers people every message type that their role may post in the
current session state (`_actions.post`): in the channel composer's type picker,
and, for a type that moves the session, as an automatic decision button. Some
types are meant only for agents or integrations, for example a `pr-event` type
that prwatch posts, or a `sized` transition that a sizer agent posts. When
people and agents share a role (for example `from: attached`), these types show
up for people. They clutter the channel and invite posts that make no sense.

Add `offer: false` under `channels.<type>.messages.<message>` in
data-entry.yaml: people do not post the type in the web app. The composer leaves
it out, it gets no automatic decision button, and it does not make Reply show.
The type stays postable through the CLI, MCP and the HTTP API, its messages show
in the channel as usual, and an explicit `button:` preset still shows (with a
load warning).

A decision can be human judgement or agent to agent; who decides is a property
of the message type, so the key sits on the type. The workaround today is a
separate agent role in acl.yaml. That works, but it is heavy when the only goal
is a cleaner UI for people.

This is presentation, not access control: the type is still in `_actions.post`,
and a person can still post it through the API.
