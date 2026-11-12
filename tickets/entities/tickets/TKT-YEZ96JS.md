---
id: TKT-YEZ96JS
type: ticket
title: 'archplan weekly: trace batch, reconcile, conflicts and flows'
kind: chore
priority: medium
effort: s
started: "2026-11-12"
completed: "2026-11-12"
status: done
---

## Description

The weekly archplan scan (GitHub issue 2164) reported four `security.untraced`
findings and two worse ratchet metrics.

## Cause

- The seqtrace routes suite sent no request to `POST /api/v1/_batch`, `POST /api/v1/_reconcile`, or a real merge conflict. So `handleV1Batch`, `handleV1Reconcile`, `conflictReadableOr404` and `conflictFieldDenial` never ran in a trace.
- `diagram.Call.Title` named a trace after the deepest call matching `\.handle`. A closure defined in the handler (`handleV1GetEntity.func1`) matched too, so archplan lost the entry point of four traces.
- A ratchet tighten on 2026-11-06, with no reason, came from a static model. It cut `weight.security` from 2300 to 140 and `weight.structure` from 936 to 90. Both need traces, so every weekly run read as worse.

## Fix

- Routes suite: add a flows.yaml, grant `git:sync` to the manager, write a conflicted task file, and send requests for conflict detail and resolve, batch, reconcile, view entries, and flows. Each includes a refused case where the route can refuse, and every new request checks its status. A demo role with `git:sync` that reads only tasks shows the conflict row gate refusing.
- `Title` skips closures of every handler it already chose.
- Accept three `untested_denial` findings that never refuse by design (dry-run create, entity help, reconcile), and re-record the ratchet from the traced model.
