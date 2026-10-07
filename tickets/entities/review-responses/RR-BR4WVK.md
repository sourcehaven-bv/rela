---
id: RR-BR4WVK
type: review-response
title: Copied config files get wider permissions
finding: Config files and directories were written with fixed modes (0644/0755 or 0600/0700) instead of the source's.
severity: minor
resolution: copyEntry keeps the source's permission bits; the private mode now caps them. TestRun_KeepsOwnerOnlyConfig.
status: addressed
---
