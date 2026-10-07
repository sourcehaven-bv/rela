---
id: RR-Y84F1C
type: review-response
title: Audit failure is invisible
finding: audit.Filesystem.Record logs and drops a failed write so the import could finish without its audit record.
severity: minor
resolution: recordAudit reads the record back and fails the run (which removes the target) when it is missing.
status: addressed
---
