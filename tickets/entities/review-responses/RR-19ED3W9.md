---
id: RR-19ED3W9
type: review-response
title: Strict schema compatibility
finding: OpenAI strict mode needs additionalProperties false and every field required (null for no change). Go-side validation stays authoritative for providers that ignore the schema.
severity: minor
resolution: 'llmtask derives a strict schema: additionalProperties false, every key required, null meaning no change. llmtask.Task.Parse re-validates every reply in Go.'
status: addressed
---
