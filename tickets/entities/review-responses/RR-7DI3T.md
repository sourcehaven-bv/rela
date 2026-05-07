---
id: RR-7DI3T
type: review-response
title: 'AC #11 (no cardinality enforcement) test calls analyze_cardinality which is an MCP tool, not testable from Go'
finding: |-
    AC #11: 'Test: Go test verifying the PATCH succeeds and asserting analyze_cardinality would now report a violation.' analyze_cardinality is exposed via internal/mcp/. From a Go test, you'd have to spin up the MCP server and call the tool over its protocol, or duplicate the cardinality-checking logic. Both are fragile and don't actually test the AC.

    Fix: drop the analyze_cardinality assertion. AC #11 becomes: 'PATCH with data: [] against a relation declaring min_outgoing: 1 succeeds and removes all edges of that type. Post-state has zero such edges.' Verify via graph.OutgoingEdges filtered by type having length 0. Whether analyze_cardinality later reports a violation is a property of the analyze tool, not the write path; test it in the analyze tool's own test suite.
severity: minor
resolution: Cardinality enforcement is now fully OUT of scope (per user direction). The corresponding AC was removed entirely; not just the analyze_cardinality assertion. The plan documents that constraints stay informational, surfaced via analyze tools.
status: addressed
---
