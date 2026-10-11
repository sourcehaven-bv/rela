---
id: RR-8JFKGU
type: review-response
title: No Counting budget test for the backlinks read path
finding: CLAUDE.md requires each new read path to pin its store-call count at 10 and 50 rows; the only Counting test covered MentionSources calls, not the request.
severity: significant
resolution: TestQueryBudget_MentionedByIsSizeIndependent drives the REST handler at 10 and 50 mentioning tickets and pins 5 reads (address, one index batch, one header batch, membership walk).
status: addressed
---
