---
id: RR-WOWGIA5
type: review-response
title: MCP shutdown after a reload dropped the last hints
finding: MCP shutdown runs current.CloseAssembly (detaching the only endpoint) before origin.Close, whose flush then had no endpoint.
severity: significant
resolution: Each assembly now flushes queued hints in stopBackgroundServices before it detaches its endpoint (retireChannelHints). The current assembly's retirement therefore sends the last hints through its own endpoint.
status: addressed
---
