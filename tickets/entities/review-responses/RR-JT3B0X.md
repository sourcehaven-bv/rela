---
id: RR-JT3B0X
type: review-response
title: CSRF doc overstated the browser guarantee
finding: docs/server-security.md said a browser always sends Origin or a cookie or Sec-Fetch-Site; old Safari with no cookie sends none (raised by both reviewers).
severity: nit
resolution: 'Reworded: every current browser sends Sec-Fetch-Site.'
status: addressed
---
