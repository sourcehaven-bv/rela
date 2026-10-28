---
id: TKT-QYFLRH
type: ticket
title: 'Session composer: offer only message types that apply in the current state'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

In a session channel the composer offers every message type the user may post,
in every state. In plan-review the owner sees abandon, accept, allow, answer,
approve, deny, go, retry, revise, rework and text; only approve and revise apply
there. A message type's `when` reads only the message, so config cannot express
this.

Let a message type declare the session states where it may be posted, or let the
composer show only types that move the session now (plus types marked always-on,
such as text).

Reported by the agent-runner PoC (rela-agent-runner-poc/rela-mission),
2026-10-28.
