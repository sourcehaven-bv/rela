---
id: RR-5PJYOQY
type: review-response
title: No test of a second page in one-shot mode
finding: The one-shot path prepends the exec mode to the args; a later page also carries keyset args and was untested.
severity: minor
resolution: 'TestListRelations_LargeIDBatchPages/pages_past_the_batch reads 1,200 rows through 600 ids: three one-shot pages with every row once.'
status: addressed
---
