---
id: RR-44NJG
type: review-response
title: Config loading must reject duplicate task names
finding: The state file keys by task name, but the config loading has no validation for duplicate names. Two tasks with the same name would silently share state, leading to incorrect missed-run detection. Config validation should reject duplicate names at startup.
severity: minor
resolution: Updated plan to validate duplicate task names at config load time
status: addressed
---
