---
id: TKT-JJRVX9
type: ticket
title: Plumb AutomationName through automation.Result for per-action audit attribution
kind: enhancement
priority: medium
effort: s
status: done
---

## Description

Cascade-created relations and entities from non-scripted automation actions
carry the generic `triggered_by: "automation"` in the audit log. Scripted
actions (`lua:` blocks) already carry the specific `automation:<name>`, because
`LuaToExecute.AutomationName` is plumbed through
`autocascade.Runner.executeScriptActions`.

The non-scripted paths drop the originating automation's name because
`automation.Result.RelationsToCreate` (a bare `[]*entity.Relation`) and
`automation.EntityToCreate` do not carry it. Operators filtering the audit log
cannot reconstruct which automation caused which cascade-created relation or
entity in a project with multiple `on: created` rules.

GitHub issue #764.
