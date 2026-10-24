---
id: RR-IM5I5R
type: review-response
title: X-Forwarded-Proto list falls back to http
finding: A header such as 'https,http' yields an http url.
severity: minor
resolution: Not changed; see reason.
reason: requestBaseURL follows the rule feedBaseURL and appBaseURL already use; the production proxy sends a single value. Changing it belongs with those helpers.
status: wont-fix
---
