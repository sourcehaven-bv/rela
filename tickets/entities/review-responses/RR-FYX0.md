---
id: RR-FYX0
type: review-response
title: Breaking change to validation.New() signature
finding: 'The plan proposes changing `validation.New(meta)` to `validation.New(meta, ws, projectRoot)`. This is a breaking change that affects all callers (workspace/analysis.go lines 345, 353). The plan should either: (1) use functional options pattern `validation.New(meta, validation.WithWorkspace(ws))` for backwards compatibility, or (2) explicitly document this as acceptable since there are only 2 call sites in the same package.'
severity: significant
resolution: 'Use functional options pattern: New(meta) unchanged, add WithWorkspace(ws) and WithProjectRoot(root) options'
status: addressed
---
