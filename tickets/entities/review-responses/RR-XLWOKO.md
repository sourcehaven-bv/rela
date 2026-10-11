---
id: RR-XLWOKO
type: review-response
title: no-cache on the spec without a validator
finding: The comment promised revalidation but the response has no ETag so every revalidation is a full download.
severity: minor
resolution: 'Comment corrected to say a reuse costs a full fetch. No ETag added: restish caches the spec and refreshes only on api sync.'
status: addressed
---
