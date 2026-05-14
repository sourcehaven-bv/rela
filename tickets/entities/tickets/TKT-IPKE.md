---
id: TKT-IPKE
type: ticket
title: Move entitymanager engine/cascade construction into a helper to trim workspace.mayDependOn
kind: refactor
priority: low
effort: s
status: backlog
---

## Summary

Workspace's `newWorkspace` directly constructs
`automation.NewEngineFromMetamodel(meta.Automations)` and
`autocascade.New(autocascade.Deps{Engine: engine})` to populate
`entitymanager.Deps`. As a result, `workspace.mayDependOn` still includes
`automation` and `autocascade` even though Manager owns the write path.

Surfaced by architect review on TKT-IU2S (#9).

## Approach

Add `entitymanager.NewWithMetamodelAutomations(deps Deps) (*Manager, error)`
that builds engine + cascade internally when `deps.Meta.Automations` is
non-empty and `deps.Automations` is nil. Workspace calls that helper instead of
constructing engine/cascade itself. Then `workspace.mayDependOn` drops
`automation` (autocascade stays because `wsScriptRunner` references
`autocascade.ScriptAction`).

## Out of scope

- Changing existing callers of `entitymanager.New(deps)` who supply
their own engine.
