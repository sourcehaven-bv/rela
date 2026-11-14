---
id: RR-WXU5N4B
type: review-response
title: Exact URL match was case-sensitive on host and port; prefix crosses the host
finding: https://API.example.com and an explicit default port did not match an exact url stub; url_prefix without a slash also matches a longer host.
severity: minor
resolution: 'Exact url compares hostnames case-insensitively and treats default ports as equal (tested). The prefix behaviour is documented in the module godoc: end a prefix with /.'
status: addressed
---
