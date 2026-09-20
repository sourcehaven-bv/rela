---
id: RR-UF6TCA
type: review-response
title: Store.Tx view dropped searchTitles
finding: The transaction view is built field by field and did not carry the new field, so a search inside a transaction fell back to prefix ranking.
severity: significant
resolution: The view carries searchTitles, with a comment naming which field is deliberately not carried.
status: addressed
---
