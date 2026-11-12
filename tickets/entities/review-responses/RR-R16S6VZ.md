---
id: RR-R16S6VZ
type: review-response
title: A webhook error can log a token in the URL query
finding: url.Error quotes the full URL, so a token an operator put in the webhook query string reached the log.
severity: minor
resolution: post unwraps url.Error and names the host only; the guide tells operators to keep tokens out of the URL and sign with secret:. TestPost_NetworkErrorRetries checks the query is not in the error.
status: addressed
---
