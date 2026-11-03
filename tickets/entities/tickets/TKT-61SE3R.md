---
id: TKT-61SE3R
type: ticket
title: 'PoC: use session default_in in the run session'
kind: enhancement
priority: low
effort: s
status: backlog
---

## Description

Add default_in: keyed (TKT-S5OJ7S) to the run session in rela-agent-runner-poc
realtime.yaml. Note that the ready trigger's go post is then skipped with a
warning outside the new state.
