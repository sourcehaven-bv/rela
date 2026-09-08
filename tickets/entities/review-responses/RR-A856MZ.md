---
id: RR-A856MZ
type: review-response
title: Free-text query branch is relevance-capped before the condition runs
finding: 'executeQueryPrefiltered documented the free-text branch as equivalent, only unpushed. It is not: that branch caps at maxFreeTextSearchResults (and the searcher limit) BEFORE nextaction.applyCondition runs, so a per-user condition on a free-text next-action query could silently under-report — the same lossy shape the condition-before-cap rule in internal/nextaction exists to prevent.'
severity: significant
resolution: 'conditionlint.conditionEntityTypes now refuses a condition on a query with free text at load (after the no-type check), with a message pointing at prop: filters; TestCompileNextActions_RefusesFreeTextQuery. executeQueryPrefiltered''s comment states the cap and the refusal.'
status: addressed
---
