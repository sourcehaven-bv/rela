---
id: RR-EI09E10
type: review-response
title: Run store error text reaches the rename caller
finding: The guard wrapped the raw run-store error, which MCP and the CLI print, and can carry database detail.
severity: minor
resolution: The cause goes to slog; the caller gets ErrRenameBlocked with a fixed message. TestFlowRenameGuard pins it.
status: addressed
---
