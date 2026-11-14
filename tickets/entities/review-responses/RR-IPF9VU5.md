---
id: RR-IPF9VU5
type: review-response
title: Key property must be unique or external_ref
finding: A non-unique key makes matches ambiguous and concurrent imports duplicate.
severity: significant
resolution: 'Validation refuses a key property that is neither unique: nor external_ref. Two stored rows with one key are a record error.'
status: addressed
---
