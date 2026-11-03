---
id: RR-ZML3XZ
type: review-response
title: 'Config-error toast is wrong for a realtime reload that breaks channels:'
finding: 'realtime_wiring.go:109-113 sends config-error naming data-entry.yaml after realtime.yaml WAS applied. useEvents.ts:191-203 always says ''Configuration not reloaded; the previous version is still in use.'' Here nothing was refused: realtime.yaml is in force, and data-entry.yaml is in force but now inconsistent. The operator is told the opposite of what happened. Add a field to the frame (for example `applied: true` or a distinct event) and word that case ''realtime.yaml applied; data-entry.yaml channels: no longer matches: ...''.'
severity: minor
resolution: The config-error toast uses the new applied field to say realtime.yaml was applied while the previous data-entry.yaml stays in use, with a test.
status: addressed
---
