---
id: TKT-096OKL1
type: ticket
title: 'Run set: offers on next-action suggestions'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

A next-action `set:` offer passes config validation but renders no button:
`NextActionOffers.vue` filters it out because no endpoint runs it. Operators who
want a one-click "Gereed" or "Beoordeeld" must write a Lua action instead of
`set: {status: gereed}`.

Found while proposing next-action config for Atlas (incident assessment, overdue
tasks, stuck NCRs).

## Acceptance criteria

- A `set:` offer renders a button and applies its properties to the suggested entity through `entitymanager.Manager.PatchEntity`, with the normal write ACL and audit.
- Values can name today (for a date such as `beoordeeld_op`).
- `confirm: true` asks first.
- A bare id resolves through `WriteTarget`; an ambiguous face is an error, not a guess.
- The suggestion refreshes after the write.
