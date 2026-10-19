---
id: TKT-R0Z2G4
type: ticket
title: 'Realtime 3/5: entity-change triggers and auto lifecycle'
kind: enhancement
priority: medium
effort: m
started: "2026-10-17"
completed: "2026-10-19"
status: done
description: Entity-change triggers (enter, leave, change) evaluated on the live tick with batch reads and a state.KV seen-set, posting system messages; lifecycle auto.when on the same engine. No Lua.
---

## Description

Entity-change triggers (enter\, leave\, change) evaluated on the live tick with
batch reads and a state.KV seen-set\, posting system messages; lifecycle
auto.when on the same engine. No Lua.

Plan: .ignored/phase3-plan.md section 6 (T5) and the decisions appended at the
end.
