---
id: RR-GLJ6GBS
type: review-response
title: A datetime literal against a date property was truncated
finding: '''2026-12-31T10:00:00Z'' compiled to 2026-12-31 silently.'
severity: minor
resolution: The compiler refuses a literal that is not a UTC midnight; test case 'datetime literal'.
status: addressed
---
