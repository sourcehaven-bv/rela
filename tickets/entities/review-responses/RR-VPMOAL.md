---
id: RR-VPMOAL
type: review-response
title: New unset-world refusals had no tests
finding: search.ValidateQuery, mcp Deps.validate and Resolver.InWorld refusals were untested (cranky review).
severity: significant
resolution: Added UnsetWorldIsInvalid subtests to storetest search and visiblesearch, a World row to the mcp Deps table, TestResolver_InWorldRefusesAnUnsetWorld, TestLinearSearch_UnsetWorldIsInvalid and a NewVisibleTracer refusal check.
status: addressed
---
