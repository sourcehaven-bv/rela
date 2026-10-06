---
id: TKT-WVSTXC
type: ticket
title: 'Co-editing: cross-node relay over postgres'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

Relay co-editing updates between `rela-server` nodes on the postgres backend, so
two users on different nodes share one room without sticky routing (RES-L4FVT0
phase 3).

## Approach

A `collab_updates` table in the tenant schema plus `NOTIFY` carrying a row id
(payloads are capped at 8000 bytes), following the change-feed pattern
(TKT-WZYWM9). Implement as a `ygo` `cluster.Relay`. Rows are ephemeral and
pruned when a room closes.
