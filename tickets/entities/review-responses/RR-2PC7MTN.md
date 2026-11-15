---
id: RR-2PC7MTN
type: review-response
title: meta.total means two different things
finding: For a structured query meta.total is the full match count; for free text it is the length before the cut, which the relevance cap (freeTextLimit) already bounds, so it can equal the rows served.
severity: significant
resolution: 'Addressed: searchPage''s doc states the total is every match for a planned query and a lower bound for free text; the PR body and ticket record it.'
status: addressed
---
