---
id: TKT-21G6JS
type: ticket
title: Agent activity indicator on channels and boards
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

People should only see what needs their action, plus a sign that the agent is
busy.

- An agent process signals "working on it" in the channel (typing over MCP, or an activity signal), delivered across processes (depends on TKT-YK30D1).
- Board cards, and maybe list rows, show an animated busy indicator when the anchored session's status is `working`. Board and list reads have no session state today; add it without per-row lookups (collection-reads rule).

Reported by the agent-runner PoC (rela-agent-runner-poc/rela-mission),
2026-10-28.
