---
id: RR-DKYZN
type: review-response
title: Entry-point enumeration incomplete — missing script.go, flow.go, reseal_sentinel.go, rela-desktop lifecycle
finding: Plan lists CLI/server/desktop/MCP/scheduler but omits internal/cli/script.go, internal/cli/flow.go (lua runtimes), internal/encryption/reseal_sentinel.go (calls NewLocalState directly), and doesn't specify where UserState injects in the Wails desktop lifecycle.
severity: significant
resolution: 'Updated file list in plan to include: internal/cli/script.go, internal/cli/flow.go, internal/encryption/reseal_sentinel.go, internal/desktop/ (Wails startup). Audit rule: every call to encryption.NewLocalState or every place that constructs an FSFactory must be listed.'
status: addressed
---
