---
id: RR-0YSVE5
type: review-response
title: Basecamp API base not checked for https
finding: http or mistyped base sends the bearer token in cleartext or elsewhere.
severity: minor
resolution: basecamp_api_base must be https; http only for loopback.
status: addressed
---
