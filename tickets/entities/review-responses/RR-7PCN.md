---
id: RR-7PCN
type: review-response
title: Struct comment about 'brief window' is misleading
finding: The struct doc says 'no caller reads these fields as a coherent set' — but CreateEntity does exactly that (automation + meta), and Search does too (graph + searchIdx). Rewrite the comment to state the actual invariant after fixing the critical issues.
severity: minor
resolution: 'Rewrote the struct doc to describe the actual concurrency model: `workspaceState` bundles meta/automation/searchIdx for coherent snapshots; reloadMu serializes Reload/Sync/Close; readers never block; writers rely on external serialization (App.writeMu). Explicitly documents the known graph-torn-during-reload limitation under a ''Known limitation'' header, so future readers don''t have to re-derive it.'
status: addressed
---
