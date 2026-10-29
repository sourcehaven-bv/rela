---
id: TKT-RQKKS9
type: ticket
title: 'Comments over stdio MCP: add_comment and watch on list_comments'
kind: enhancement
priority: medium
effort: m
started: "2026-10-28"
completed: "2026-10-28"
status: done
---

## Description

People want to talk to a working agent through comments on the entity, with the
agent watching them. Stdio MCP offers only `list_comments`: no `add_comment`,
and `watch` does not accept `list_comments` as a target.

Add `add_comment` (as the job identity under `--run-as`) and make
`list_comments` a watch target.

Reported by the agent-runner PoC (rela-agent-runner-poc/rela-mission),
2026-10-28.
