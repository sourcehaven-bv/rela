---
id: RR-8JQLRS2
type: review-response
title: Records card check listed forbidden fields
finding: A field added to DashboardCard later would be accepted on a line card and ignored.
severity: minor
resolution: onlyRecordsCardFields zeroes the allowed fields and requires the rest to be zero; test with tone_by added.
status: addressed
---
