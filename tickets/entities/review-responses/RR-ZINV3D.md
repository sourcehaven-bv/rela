---
id: RR-ZINV3D
type: review-response
title: Merge key smuggles locked keys past the allowlist
finding: DecodeJSON accepted '<<' as a key; written out it became a YAML merge key that grafted locked keys (export_render, scan) one level up from where the allowlist checked them.
severity: critical
resolution: DecodeJSON refuses '<<', and Apply refuses unless the written bytes parse back to the checked tree. TestMergeKeysAreRefused.
status: addressed
---
