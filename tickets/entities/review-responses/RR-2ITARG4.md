---
id: RR-2ITARG4
type: review-response
title: Error detail leaked a possibly hidden lock date
finding: checkOpen put the owner's closed_before date in the 422 detail, read from the raw entity, which may hold a visible:-hidden value.
severity: significant
resolution: 'The message no longer names the date: ''the period is closed for <id>''.'
status: addressed
---
