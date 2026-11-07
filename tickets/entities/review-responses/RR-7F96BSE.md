---
id: RR-7F96BSE
type: review-response
title: Memory build shares per-tenant state via the project cache dir
finding: OpenAndAssemble on the memory build gave each tenant a fresh memstore but piles, comments, aliases and settings fell back to Paths.CacheDir, shared by all tenants.
severity: significant
resolution: The memory build now refuses OpenAndAssemble like fs and sqlite (openassemble_local.go is tagged !postgres).
status: addressed
---
