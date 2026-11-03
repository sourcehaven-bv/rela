---
id: TKT-QR1VTP
type: ticket
title: Point to RELA_DATAENTRY_USER on acl_unstamped_principal
kind: enhancement
priority: low
effort: s
started: "2026-10-30"
completed: "2026-10-30"
status: done
---

## Description

With an `acl.yaml` and no auth configured, every request fails with
`acl_unstamped_principal`, and nothing points to `RELA_DATAENTRY_USER` or the
auth settings. Add a startup warning and a hint in the error detail.

Reported by the agent-runner PoC (rela-agent-runner-poc/rela-mission),
2026-10-28.
