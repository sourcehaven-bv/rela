---
id: RR-HUBIOD
type: review-response
title: Page shape revealed roughly how many hidden sources exist
finding: A short or empty page that still carried a cursor, and an attacker-chosen cursor, let a caller binary-search where hidden mentioners lie in id order.
severity: minor
resolution: The cursor can only name a served id, so a caller cannot move the scan window through hidden rows. The one remaining signal, truncated when the 10,000-row scan budget is used up, is documented as accepted in docs/acl-security.md.
status: addressed
---
