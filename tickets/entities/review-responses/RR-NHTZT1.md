---
id: RR-NHTZT1
type: review-response
title: The PR job kept the token in .git/config while running PR code
finding: 'The checkout persisted credentials with security-events: write while the export ran the PR''s code.'
severity: minor
resolution: 'The checkout sets persist-credentials: false.'
status: addressed
---
