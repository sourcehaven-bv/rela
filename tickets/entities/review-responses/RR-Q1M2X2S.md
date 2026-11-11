---
id: RR-Q1M2X2S
type: review-response
title: A lookup error on the write path reads as a user denial
finding: Fail-closed read-only turns a run-store error into a 403 and an audit row that blames the user.
severity: minor
resolution: 'Addressed in plan: the attribution names the lookup failure (outside_edits:unavailable) so the audit row is not misread; the response stays a refusal (fail closed).'
status: addressed
---
