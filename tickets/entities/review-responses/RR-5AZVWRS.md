---
id: RR-5AZVWRS
type: review-response
title: Doc comment says quotes but only single quotes are blocked
finding: 'filters.ts: the comment says quotes, but the regex blocks only the single quote, as Go does.'
severity: nit
resolution: Comment now says single quotes; a test keeps a name with double quotes.
status: addressed
---
