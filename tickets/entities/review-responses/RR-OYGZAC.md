---
id: RR-OYGZAC
type: review-response
title: Copied project files are not verified; uint64 above MaxInt64
finding: Project files are not re-hashed in the target; a uint64 above MaxInt64 passes normalization and fails verify with a vague message.
severity: nit
reason: Project files are copied byte for byte inside the fingerprinted window. YAML integers above MaxInt64 do not occur in rela properties; verify still refuses them.
status: wont-fix
---
