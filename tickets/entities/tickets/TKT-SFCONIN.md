---
id: TKT-SFCONIN
type: ticket
title: 'rela test: advisory hints that nudge towards generated test data'
kind: enhancement
priority: low
effort: m
status: backlog
---

## Description

Advisory hints (never failures) printed by `rela test`:

- **Literal timestamps** in a test file (ISO dates in string constants) → suggest `t.now()` plus a duration.
- **Properties set but never read or asserted** in a case → suggest letting the faker fill them. Needs the execution trace.
- **Hand-written principal names** that never matter to an assertion → suggest `t.principal{roles}`.
- **Markdown fixture directories** used only to provide filler entities → suggest generated graphs.

`--strict-hints` turns the hints into failures for teams that want to enforce
the style.
