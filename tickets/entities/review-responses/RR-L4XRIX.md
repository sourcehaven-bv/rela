---
id: RR-L4XRIX
type: review-response
title: mcp_wiring glob matches only a test file
finding: internal/cli/mcp_wiring_*.go matched just mcp_wiring_test.go.
severity: minor
resolution: Removed the glob; the guard now ignores _test.go matches so this cannot recur.
status: addressed
---
