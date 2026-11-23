---
id: RR-YNSWLUM
type: review-response
title: Step bound goes stale when data-entry.yaml hot-reloads
finding: '[security] internal/appbuild/flowruntime.go sets s.flowForms once at engine build; the data-entry watcher reloads data-entry.yaml without rebuilding it. After a hand edit that removes a field the server still accepts it from the API/MCP; an added field renders read-only; a removed form reloads in the SPA but stops the server on next start. Fix: resolve the bound per lookup from the current data-entry config (or rebuild on reload) and refuse a reload that breaks a step form.'
severity: significant
resolution: FlowControl reads step forms through stepFormSource from data-entry.yaml once per operation; s.flowForms is gone. An edit applies to the next step without a restart. Pinned by TestFlowControl_StepFormReadPerStep.
status: addressed
---
