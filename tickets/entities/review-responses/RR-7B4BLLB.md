---
id: RR-7B4BLLB
type: review-response
title: 'Design: rows loaded for completion skip the row gate'
finding: A row listed while readable and changed to unreadable before the load would be redacted and hashed and folded; the count must also drop for rows dropped at load.
severity: significant
resolution: Loaded entity rows are checked again against the type plan gate (one MatchingFaces per gated type) and face allowlist; rows that fail or are gone are dropped and the count is taken from the folded rows. Relation NULL-element rows keep the listing's endpoint verdict; documented.
status: addressed
---
