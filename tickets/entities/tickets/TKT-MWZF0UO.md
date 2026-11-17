---
id: TKT-MWZF0UO
type: ticket
title: done_when per step
kind: enhancement
priority: medium
effort: m
started: "2026-11-15"
completed: "2026-11-15"
status: done
---

## Description

Follow-up to TKT-35BY3PE (workshop step mode). An optional `done_when` predicate
per tab on a steps page marks the step done, for example "every risk has an
owner". The step bar shows a check on the step number.

- Evaluates through `internal/predicate` (`.claude/rules/predicate.md`),
compiled at config load like other view conditions.
- Advisory only: it never blocks Next.
- One gated query per step, no per-row lookups
(`.claude/rules/collection-reads.md`).
- Per principal, evaluated over the gated set only, so a hidden row never
changes the answer.
