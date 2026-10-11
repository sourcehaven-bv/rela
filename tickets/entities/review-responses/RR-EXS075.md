---
id: RR-EXS075
type: review-response
title: MCP reload waits for the old sweep under the mutex
finding: old.CloseAssembly() runs under s.mu and Stop waits unbounded for the in-flight tick.
severity: minor
resolution: reload and Close publish or capture under s.mu and tear down after unlocking.
status: addressed
---
