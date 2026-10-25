---
id: RR-Y9KYO5
type: review-response
title: Issue dedup trusts any issue with the scan title
finding: The title-only dedup matched any open issue with that title; anyone could open one and capture or bury the scan reports. TITLE was also spliced into the jq program.
severity: significant
resolution: 'Dedup now also requires the author to be app/github-actions or app/rela-coverage-bot; TITLE is passed to a standalone jq via --arg. Verified: the live query returns 1986 and an issue by another author is not matched.'
status: addressed
---
