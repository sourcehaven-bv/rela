---
id: TKT-852C0M
type: ticket
title: Trigger and reminder literals through MessageType.NormalizeProps
kind: chore
priority: low
effort: s
started: "2026-10-31"
completed: "2026-10-31"
status: done
---

## Description

TKT-S9TLJJ moved message prop rules into realtimecfg as
MessageType.NormalizeProps. Trigger and reminder literals (literalMessage) still
check props with their own copy of the rules. Route them through NormalizeProps
so there is one copy; refusal messages for those literals will change.
