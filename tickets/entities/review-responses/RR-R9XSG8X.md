---
id: RR-R9XSG8X
type: review-response
title: ByIDs read lacks face and world coverage
finding: No test covered the id read on a faced type under a world or a face allowlist, and the both-branches table had no ByIDs row.
severity: significant
resolution: Added TestScopedHeaders_ByIDsKeepsTheWorldsFace (world prime, one of two ids, allowlist falls through the chain, both verdict branches, compared with the full read) and two ByIDs rows in TestScopedHeaders_NarrowingsApplyToBothVerdictBranches.
status: addressed
---
