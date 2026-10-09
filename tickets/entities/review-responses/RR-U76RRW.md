---
id: RR-U76RRW
type: review-response
title: 'Security: audit does not record who triggered a job'
finding: triggered_by named only the automation.
severity: minor
resolution: The payload carries the saver's user as data; the label is automation-job:<name>;by=<user>.
status: addressed
---
