---
id: RR-UOKCUVQ
type: review-response
title: Wrong-typed stub keys were ignored
finding: body = 123 dropped the body condition and widened the stub; a non-string response body or file gave an empty 200.
severity: minor
resolution: 'optString rejects any non-string value for method, url keys, body, name, error and file. Test: body not a string.'
status: addressed
---
