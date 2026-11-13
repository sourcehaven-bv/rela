---
id: RR-EUNN5FP
type: review-response
title: Request-served reports labelled batch-job contradict requestPath definition
finding: /_analyze and MCP analyze tools run in a request but are batch-job.
severity: minor
resolution: The batchJob comment now says a whole-graph report served on request is a batch job because its output is about every row; requestPath now names a request whose answer is a page or a few rows.
status: addressed
---
