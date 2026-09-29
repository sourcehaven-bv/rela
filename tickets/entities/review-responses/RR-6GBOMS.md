---
id: RR-6GBOMS
type: review-response
title: Orphan JSON contract changed and the title rule differed per surface
finding: rela analyze orphans -o json now emits {id,type,title,faces} rather than full entities, and a faced orphan had no title in the CLI while MCP showed the world-served title.
severity: significant
resolution: tracer.Orphan now carries the title of the face its world serves (redacted in VisibleTracer, one batched read), and analysis uses it, so the CLI matches rela trace. The JSON shape and per-check coverage are documented in docs/cli-reference.md under rela analyze.
status: addressed
---
