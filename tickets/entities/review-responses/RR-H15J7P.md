---
id: RR-H15J7P
type: review-response
title: Grep guard for mode switches would pass vacuously
finding: After the move a source guard finds nothing, so a broken pattern would still pass.
severity: minor
resolution: internal/archguard/aclmode_test.go scans the tree with an allowlist (access/mode.go and the rela-server startup warning) and TestACLModeChecks plants positive and negative cases.
status: addressed
---
