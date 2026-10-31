---
id: RR-8URBLSF
type: review-response
title: TestGoldenParity fails on 19 goldens
finding: 'Code review: The analyze, document, export-document and mcp-list goldens for decl-header and decl-jwt changed their reads lines.'
severity: critical
resolution: Regenerated with -update. Only reads, breadth and bodies lines changed; response bodies are identical. Document reads drop from one GetRelation per row to two ListRelations.
status: addressed
---
