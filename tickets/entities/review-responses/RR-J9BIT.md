---
id: RR-J9BIT
type: review-response
title: stdout/stderr accumulated but never surfaced on failure
finding: Server stdout/stderr are collected into strings but never attached to test results. Failures lose correlatable server logs. Also unbounded string growth per test.
severity: significant
resolution: serverUrl fixture now takes testInfo; on test failure it attaches server stdout/stderr as 'rela-server.log'. On success the logs are discarded.
status: addressed
---
