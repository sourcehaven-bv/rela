---
id: RR-VS01Y2M
type: review-response
title: storedHeaders mislabelled and bounded/passThrough overlap
finding: Helpers taking a query parameter were labelled bounded although the caller builds the scope.
severity: minor
resolution: 'Resolved by the helper propagation: those entries are gone and their callers are listed. passThrough now only covers helpers that change their caller''s query first (listGatedThenRanked, listPushdown).'
status: addressed
---
