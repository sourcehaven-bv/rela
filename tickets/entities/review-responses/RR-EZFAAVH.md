---
id: RR-EZFAAVH
type: review-response
title: Priming flow runs in policyResolver.PrimeTraversals makes redact-only reads pay
finding: dataentry affRedactor.PrimeTraversals calls primeVerdicts which calls policyResolver.PrimeTraversals, so every redact-only read would pay a run query.
severity: significant
resolution: 'Addressed in plan: a separate affordances.PrimeFlowRuns, called only where per-row _fields are built (view sections and editable table rows).'
status: addressed
---
