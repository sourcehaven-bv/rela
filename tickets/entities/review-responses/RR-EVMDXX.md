---
id: RR-EVMDXX
type: review-response
title: Log-capture test swapped the global slog default inside a package full of parallel tests
finding: TestHandleAnalyzeCardinality_TruncatedScanIsLogged called slog.SetDefault to capture the checker's warning. It correctly omitted t.Parallel and restored via t.Cleanup, but package mcp has 127 parallel tests; any of them emitting a slog record during the window would race on the default logger, and the race detector is on in CI. It would surface as a flake attributed to an unrelated test.
severity: minor
resolution: |-
    Moved the log assertion to internal/schema/cardinality_test.go, where the behavior actually lives and where no test in the package calls t.Parallel — with a comment saying to keep it that way. The MCP test is renamed TestHandleAnalyzeCardinality_TruncatedScanStillAnswers, keeps t.Parallel, and asserts only the MCP-level contract (answer, don't fabricate).

    Not done: threading a *slog.Logger into the checker. internal/analysis uses package-global slog.Warn in five places for exactly this class of diagnostic, so a logger parameter on one sibling function would diverge from the convention rather than fix it.
status: addressed
---
