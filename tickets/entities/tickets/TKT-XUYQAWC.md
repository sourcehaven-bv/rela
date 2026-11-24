---
id: TKT-XUYQAWC
type: ticket
title: Per-type records:read and records:append permissions
kind: enhancement
priority: medium
effort: m
started: "2026-11-23"
completed: "2026-11-23"
status: done
---

## Description

Split from TKT-EV8U8T4. Optional `records:read` and `records:append` permissions
per record type. Today access follows the owner: read the owner to read its
rows, update every face of it to append.

Decide whether a permission narrows the owner rule or replaces it. Per
`acl-ceiling.md` it must compile to an allowlist when `acl.yaml` loads, never a
runtime deny.

## Acceptance

- A type with `records:append` refuses an append by a principal without it,
even when that principal may update the owner.
- Every surface (REST, Lua, MCP, CLI) applies it through `records.Service`.
