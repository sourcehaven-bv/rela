---
id: RR-GY2RRI9
type: review-response
title: has_more is wrong for a limited free-text page
finding: With a limit, freeTextLimit caps the hits at offset+limit, so total equals offset+limit when the page fills and has_more = offset+len < total is false even when more hits exist.
severity: significant
resolution: 'Addressed: the free-text and Go paths ask for offset+size+1 rows, so a further row shows as total > offset+size. The total stays a lower bound for free text, as before.'
status: addressed
---
