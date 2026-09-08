---
id: RR-7LL8MB
type: review-response
title: Dead resolveRelationColumnValues left behind after section batching
finding: No non-test callers remained, and the wrapper built a row entity with an empty Type that would mis-resolve direction if called.
severity: significant
resolution: Deleted. The tests that used it now go through a test-only helper over resolveRelationColumns with the row's type supplied.
status: addressed
---
