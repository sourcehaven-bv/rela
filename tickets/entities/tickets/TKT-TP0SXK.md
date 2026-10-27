---
id: TKT-TP0SXK
type: ticket
title: Channels menu entry with session_state filter
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

A data-entry `channels:` menu entry that lists channels across entities, with a
`session_state:` filter (for example all incidents waiting in `contained`).
Split off from TKT-IJM06E (Sessions 4/7): `data-entry.yaml` has no `channels:`
entry and no cross-entity channel list yet.

Needs an ACL-gated cross-entity channel list (row-gate anchors before counting
or paging, no per-row lookups, budget test), the menu wiring and the SPA list.
Share the store query with the inbox (TKT-ZZ4PNX).
