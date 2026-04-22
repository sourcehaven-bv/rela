---
id: RR-17XTS
type: review-response
title: 50ms post-SIGTERM sleep is insufficient
finding: proc.kill('SIGTERM') does not wait for exit; 50ms is nowhere near enough under CI load. Different worker may reuse the port before the previous server fully exits.
severity: critical
resolution: Replaced sleep(50) with waitForExit that awaits proc 'exit' event, with 5s SIGKILL escalation. See e2e/tests/fixtures.ts waitForExit.
status: addressed
---
