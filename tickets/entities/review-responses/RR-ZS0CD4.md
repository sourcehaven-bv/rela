---
id: RR-ZS0CD4
type: review-response
title: Budget test cannot catch a per-hit grant verdict
finding: TestQueryBudget_ListFreeTextUnderFieldHiding subtracted the per-hit GetEntity and its query matched only the unconditionally visible title, so a lazy per-hit MatchingFaces would still pass.
severity: significant
resolution: The test now also runs q=open, which matches only the conditionally visible status, asserts the admitted row count (n/10), and compares absolute GetEntity and MatchingFaces counts at 10 and 50 hits.
status: addressed
---
