---
id: TKT-BHYJZCV
type: ticket
title: Show the outcome of background llm action runs
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

An `llm:` action with `apply: auto` runs in the background (TKT-WQ925LL). When
the run fails (provider error, a refused write, a stale property), the failure
is logged on the server only. The user who started it sees "being made in the
background" and nothing after.

Give the user the outcome: for example a notification or an entry on the entity
that names what was written, what was skipped and why a run failed. Flow steps
already record failure on the flow run; list and detail-page runs do not.
