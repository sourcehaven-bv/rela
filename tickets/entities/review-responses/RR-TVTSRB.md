---
id: RR-TVTSRB
type: review-response
title: On sqlite the CLI and stdio MCP cannot run beside the server, so the cross-process story does not apply
finding: sqlitestore takes an exclusive sidecar lock (DEC-LFSYNY), so rela-sqlite channel and rela-sqlite mcp are refused while rela-server-sqlite or desktop runs. Say in docs/realtime.md that remote MCP is the only agent path on sqlite.
severity: minor
resolution: docs/realtime.md states that on sqlite remote MCP is the only agent path while the server runs.
status: addressed
---
