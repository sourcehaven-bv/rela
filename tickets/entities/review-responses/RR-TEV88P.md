---
id: RR-TEV88P
type: review-response
title: Date property with a time of day is truncated
finding: 'A time.Time on a date property was formatted as YYYY-MM-DD whatever its time of day; due: 2026-03-04T15:00:00 lost the time silently. Verify used the same normalization so it could not catch this.'
severity: significant
resolution: formatTime keeps a date only at exact midnight; any time of day fails the row with 'date property holds a time of day'. Pinned by TestNormalizeProps cases that do not go through verify.
status: addressed
---
