---
id: RR-OSRK3J
type: review-response
title: Rebuild cannot reuse main's os.Exit wiring
finding: About 15 wiring steps after NewApp call os.Exit (cmd/rela-server/main.go:478-560); the remote MCP factory captures the old svc (mcp.go:86-117).
severity: significant
resolution: 'Plan changed: extract buildApp(svc, proc) returning errors; process-level values (JWT verifier, flags, security config) built once in main.'
status: addressed
---
