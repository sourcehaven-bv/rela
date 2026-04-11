---
id: RR-93W7S
type: review-response
title: 'json_decode error handling: programming error vs runtime error'
finding: 'The plan says http.json_decode("invalid") should RaiseError (programming error). But this is inconsistent with the stated (nil, err_table) convention for expected failures. If a script fetches a response body and tries to decode it, invalid JSON from an external API is an expected runtime failure, not a programming mistake. Consider: RaiseError for wrong arg type (not a string), but (nil, err_table) for valid string that isn''t valid JSON. This matches how ai.chat handles malformed upstream responses.'
severity: significant
resolution: json_decode returns (nil, err_table) for invalid JSON instead of raising
status: addressed
---
