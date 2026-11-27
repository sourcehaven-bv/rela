---
id: RR-Q1N5EI6
type: review-response
title: scanHeaders q and req can disagree
finding: applyScope takes the scope from req, so a caller passing a different req to scanHeaders than to scopedQuery skips the scope silently.
severity: significant
resolution: 'Source guard test: every scanHeaders call passes the query, error class and request from one scopedQuery call in the same function, plus the runtime type check.'
status: addressed
---
