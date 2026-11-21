---
id: TKT-JBVW6PH
type: ticket
title: Named record backend registry
kind: enhancement
priority: low
effort: m
status: backlog
---

## Description

Split from TKT-EV8U8T4. A named backend registry: `.rela/records.yaml` `backend:
<name>` resolved from factories in `internal/appbuild`, with a test backend
proving selection.

Each build already picks file, sqlite or postgres records to match its store. Do
this when a real external backend (for example a time-series database) is
wanted; until then it adds a config surface with no user.

## Acceptance

- A test backend named in `.rela/records.yaml` is selected at wiring.
- An unknown backend name fails startup with a clear message.
