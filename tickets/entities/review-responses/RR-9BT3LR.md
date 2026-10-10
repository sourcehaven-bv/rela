---
id: RR-9BT3LR
type: review-response
title: 'Code: foreground semantics inconsistent'
finding: retry ignored, wrong log wording, token written without a queue, rela mcp/scheduler run in foreground.
severity: minor
resolution: Foreground skips tokens; log says 'failed'; docs state retry does not apply and that rela mcp and rela scheduler run in the foreground. Moving those to a queue is out of scope.
status: addressed
---
