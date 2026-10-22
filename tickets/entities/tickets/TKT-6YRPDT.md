---
id: TKT-6YRPDT
type: ticket
title: 'Sessions 4/6: input_required and handoff'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

`status:` per state using MCP tasks values (`working`, `input_required`,
`completed`, `cancelled`, `failed`) and `waiting_for:` naming the role or
principal; MCP `watch` on sessions. Role holders come from entity properties
(`from: entity.<prop>`); handoff is a `set:` output on that property.

Design: RES-O8I8S7 (decisions section). Part of FEAT-GWF8YH phase 4.
