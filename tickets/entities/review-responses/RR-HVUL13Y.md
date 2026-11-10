---
id: RR-HVUL13Y
type: review-response
title: 'Code review: watcher exits with os.Exit'
finding: tenantRefused called os.Exit from the watcher goroutine, skipping deferred cleanup and possibly splitting a write from its audit record.
severity: significant
resolution: tenantRefused now cancels the command context created in runKong (as SIGTERM does), so deferred cleanup runs; os.Exit only after a 30s grace period.
status: addressed
---
