---
id: RR-CE2WTD
type: review-response
title: 'Review nits: position collation note, empty-result position test, jsonIDs failure direction, display allowlist coupling, swallowed narrowing error'
finding: Five small items from the code and security reviews.
severity: minor
resolution: Window id ordering documents its reliance on the COLLATE C column; conformance gains an empty-result case; jsonIDs panics instead of returning a match-nothing value; contentFreeDisplays names the section switch it is coupled to; storePosition documents why declining on a narrowing error is safe and when it would stop being safe.
status: addressed
---
