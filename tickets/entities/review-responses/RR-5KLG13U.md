---
id: RR-5KLG13U
type: review-response
title: AI call duration and cancellation
finding: The request blocks for the provider timeout. Bound it with a capped per-task timeout and honour ctx cancellation; SPA shows progress.
severity: minor
resolution: The provider call runs under a 2-minute context timeout derived from the request context, so a cancelled request stops it. The action button shows the pending state.
status: addressed
---
