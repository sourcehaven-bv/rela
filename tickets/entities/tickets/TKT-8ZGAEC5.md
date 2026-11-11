---
id: TKT-8ZGAEC5
type: ticket
title: 'Two-way sync: latest/local/remote rules and relation push (9b)'
kind: enhancement
priority: medium
effort: m
started: "2026-11-11"
completed: "2026-11-11"
status: done
---

## Description

Step 9b of FEAT-CMVZQFI (DEC-2NFLAHB), on top of 9a (TKT-UH2Q9MN).

- Conflict rules `latest`, `local` and `remote`, per remote, type and field.
`latest` keeps the losing value on the conflict list and depends on both
machines' clocks.
- Serve the last-updated time on per-entity reads only when nothing on the
row is hidden from the reader. When it is withheld, `latest` falls back to
`hold` for that row, with an explicit reason on the conflict list.
- Push relations created locally between linked rows, with the same field
rules for relation meta.

## Acceptance

- Each rule tested per remote, type and field, in the two-instance test.
- A row with a hidden field gets no last-updated time and its conflict is
held with the reason.
