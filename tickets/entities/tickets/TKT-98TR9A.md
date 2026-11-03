---
id: TKT-98TR9A
type: ticket
title: 'Sessions 4/5a: message-driven graph outputs'
kind: enhancement
priority: medium
effort: l
started: "2026-10-25"
completed: "2026-10-26"
status: done
---

## Description

`outputs:` on transitions with `set:` and `create:` write to the graph through
entitymanager. Message-driven transitions write as the poster; timer-driven ones
as the session type's `run_as` principal granted in `acl.yaml` (a missing grant
fails at load). Pending/done marks make outputs idempotent. `on_final: close`
closes the channel.

Design: RES-O8I8S7 (decisions section). Part of FEAT-GWF8YH phase 4.
