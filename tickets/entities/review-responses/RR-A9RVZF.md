---
id: RR-A9RVZF
type: review-response
title: 405-only cases count as coverage
finding: GET _copies and GET _attachments/{property} answer 405 for all principals yet count as covered.
severity: significant
resolution: Dropped the 405-only copies and attachment cases; a case that records a 405 now fails the test.
status: addressed
---
