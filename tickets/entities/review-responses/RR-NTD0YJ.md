---
id: RR-NTD0YJ
type: review-response
title: Stdio realtime.yaml reload must target s.current() and respect the closed flag
finding: After a schema reload the successor has its own RealtimeConfig holder (newRealtimeConfig per SharedBase). The section 3.1 callback must resolve s.current() on each event and skip once mcpServices.closed is set, or it reloads a retired holder. Add that to TestMCPStdio_RealtimeReload.
severity: minor
resolution: The reload callback resolves s.current() per event and skips once closed is set. TestMCPStdio_RealtimeReload covers both.
status: addressed
---
