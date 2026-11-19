---
id: RR-7L88NSH
type: review-response
title: rela acl audit would report a builtins permission as dead
finding: dataEntryPermissions.UsedPermissions (internal/cli/acl.go) collects every UI-gate permission but not cfg.Builtins or sp.Builtins, so A7 would flag a working grant used only by builtins.analysis.permission and advise revoking it.
severity: significant
resolution: UsedPermissions now collects the top-level and per-space builtins permissions. TestDataEntryPermissions_PerSurface has a case for each.
status: addressed
---
