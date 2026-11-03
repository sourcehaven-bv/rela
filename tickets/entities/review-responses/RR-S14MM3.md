---
id: RR-S14MM3
type: review-response
title: 'issues: write was granted to every weekly step'
finding: Workflow-level permissions gave the issue scope to npm ci, run.sh and the export.
severity: minor
resolution: 'The issue step moved to a report job with issues: write only; the scan job has contents: read and security-events: write and passes a reduced report as an artifact.'
status: addressed
---
