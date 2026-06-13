---
id: RR-AJ0QD8
type: review-response
title: Bleve Limit:0 maps to a 10000 floor, not truly uncapped
finding: bleveindex.go:221 maps limit ≤0 to req.Size = 10000. The generic path silently caps candidates at 10000 before filtering on the fs/bleve backend, re-introducing the cap-starvation class at a higher bound. Plan should acknowledge and accept explicitly rather than claim 'uncapped'.
severity: minor
resolution: 'Plan rev 2: bleve''s 10000 floor explicitly acknowledged and accepted — cap starvation fixed within a 10k-candidate window on bleve, fully on linear/pgstore. Documented in GUIDE-acl-security (AC10) instead of claiming ''uncapped''.'
status: addressed
---
