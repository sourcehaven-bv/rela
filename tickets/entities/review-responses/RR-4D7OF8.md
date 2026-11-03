---
id: RR-4D7OF8
type: review-response
title: Identity cap can be raced
finding: 'Security and code review: identityTotal released h.mu before c.subs was replaced. Concurrent PUTs on one identity''s streams could each pass the cap.'
severity: minor
resolution: replace holds h.mu from the count through the commit.
status: addressed
---
