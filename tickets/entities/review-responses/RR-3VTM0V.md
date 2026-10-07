---
id: RR-3VTM0V
type: review-response
title: Unquoted date on a non-date property becomes a timestamp
finding: 'version: 2026-03-04 on a string property decoded as time.Time and was stored as 2026-03-04T00:00:00Z.'
severity: significant
resolution: A midnight-UTC time.Time on a property that is not a datetime is stored as YYYY-MM-DD. TestNormalizeProps 'bare date on a string property keeps its text'.
status: addressed
---
