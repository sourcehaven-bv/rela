---
id: RR-SQ6Z9P
type: review-response
title: cli_wiring allowlist reason omits the operator-shell trust boundary
finding: The allowlist reason for rela restore did not say that it has no field gate and relies on operator-shell trust.
severity: nit
resolution: The cli/restore.go entry now states it is an operator-shell command with no field gate, the same trust boundary as every other CLI write.
status: addressed
---
