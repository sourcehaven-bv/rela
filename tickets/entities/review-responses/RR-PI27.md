---
id: RR-PI27
type: review-response
title: Entity context globals should mirror existing Lua table format
finding: |-
    The plan proposes setting `entity` and `old_entity` as Lua globals. The Lua runtime already has `entityToTable()` function (runtime.go:396-409) that converts entities to Lua tables with specific structure (id, type, content, properties).

    **Recommendation:** Reuse `entityToTable()` or export it to ensure consistent entity representation in both automation Lua and regular Lua scripts. Currently it's unexported - may need to export or duplicate.
severity: minor
resolution: Exported EntityToTable and GoToLuaValue from lua package for use by workspace when setting up entity context.
status: addressed
---
