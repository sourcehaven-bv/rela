---
id: RR-06ZS7QG
type: review-response
title: 'Design: who can call it in v1 and CSRF'
finding: The plan does not say which identity works before step 7 or test that a cookie-bearing cross-site POST is refused.
severity: minor
resolution: v1 callers authenticate as any API client does (JWT assertion or the configured principal header); peer credentials are step 7. The CSRF exemption keeps requiring the no-cookie no-Origin shape; a router security test covers a cross-site browser POST.
status: addressed
---
