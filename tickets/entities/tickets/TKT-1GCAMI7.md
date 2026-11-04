---
id: TKT-1GCAMI7
type: ticket
title: Compute kanban column summaries on the server
kind: refactor
priority: medium
effort: s
tags: security
status: backlog
---

## Description

Kanban column summaries are computed in the SPA (`utils/summary.ts`) over the
cards it fetched. Compute them on the server over the gated card set, in the
order the gantt endpoint uses: row-gate, redact, then fold. Add an ACL test with
a hidden card whose value must not reach the summary.

Part of FEAT-QRI3CFR (step 1.2).
