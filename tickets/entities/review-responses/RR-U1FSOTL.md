---
id: RR-U1FSOTL
type: review-response
title: Paging rules need to be explicit
finding: An empty page with more=true is valid; only a last that does not advance is an error; also check last against hi and the cursor.
severity: minor
resolution: 'Paging rules are in page() and checkLast(): elements strictly increasing after the cursor; last at or past the last element and inside the range; a page with more must advance; a cursor at the bound length is refused.'
status: addressed
---
