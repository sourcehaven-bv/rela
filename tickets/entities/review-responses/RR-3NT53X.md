---
id: RR-3NT53X
type: review-response
title: Frontend code scan overrides the server's precise segments
finding: The 4-space indent and cross-block backtick regexes dropped nested list items and segments after a stray backtick.
severity: minor
resolution: 'The approximate scan now applies only to ranges without segments (older server). Test: ''trusts server segments over the approximate code scan''.'
status: addressed
---
