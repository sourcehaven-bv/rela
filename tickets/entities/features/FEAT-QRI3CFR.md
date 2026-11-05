---
id: FEAT-QRI3CFR
type: feature
title: Server-resolved views for native and TUI clients
summary: The server sends resolved views so the SPA and future native and TUI clients only render
description: Move config execution from the SPA to the server so the SPA and future native and TUI clients render resolved views instead of interpreting data-entry.yaml.
priority: medium
status: proposed
---

## Description

Prepare rela for native clients (iOS/macOS, Android) and a TUI by having the
server send resolved views instead of config plus raw data. Today the SPA
executes much of `data-entry.yaml` itself: kanban grouping and drops, list
groups, date buckets, calendar mapping, default widgets, form steps and
conditions. Every new client would have to reimplement that.

Each ticket moves one piece to the server, keeps the SPA's behaviour the same,
and deletes the client logic in the same change. The plan has five parts:
contract groundwork (generated wire types, additive-only check), small read-side
moves, a kanban endpoint with intents, resolved forms with portable conditions,
and a TUI spike that proves the contract is browser-neutral.

Stays on the client: gestures, locale formatting, the decimal edit preview,
router state, and the autosave conflict UI.

Research: RES-P8O4FND.
