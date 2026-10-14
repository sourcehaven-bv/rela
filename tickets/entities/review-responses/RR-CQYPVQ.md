---
id: RR-CQYPVQ
type: review-response
title: Link host may hold markdown or HTML
finding: Go accepts hosts such as a)b and x<details>, which close a link destination or open raw HTML.
severity: critical
resolution: plainHost accepts only a DNS name or an IP literal with a numeric port; test cases added.
status: addressed
---
