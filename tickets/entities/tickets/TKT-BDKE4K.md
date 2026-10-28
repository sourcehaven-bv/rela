---
id: TKT-BDKE4K
type: ticket
title: 'rela validate accepts deprecated link: true that rela-server refuses'
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Description

`link: true` passes `rela validate`, and the data-entry quick start uses it, but
rela-server refuses it as deprecated. Make validate report it, and fix the quick
start.

Reported by the agent-runner PoC (rela-agent-runner-poc/rela-mission),
2026-10-28.
