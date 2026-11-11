---
id: TKT-IVL2LF9
type: ticket
title: 'Flows M1i: waiting runs in _inbox'
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Description

Split from TKT-FWQZGAM. Show the runs that wait for a role the reader holds in
`_inbox`, read through the header's visibility gate (RR-R5I7UE0).
`appbuild.FlowControl.Tasks` already lists them through that gate, so `_inbox`
can reuse it.

## Acceptance

- A waiting run shows in the inbox of a person who holds a role it waits
for, and never in the inbox of someone who cannot read its header.
