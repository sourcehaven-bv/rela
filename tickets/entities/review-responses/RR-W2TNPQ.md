---
id: RR-W2TNPQ
type: review-response
title: DefaultMatch returned a template, so record data could skip flattening
finding: DefaultMatch returned a template, so record data could skip flattening
severity: minor
resolution: Source.DefaultMatchRef returns a reference. ingest wraps it, so its value is resolved and flattened. TestRun_DefaultMatchValueIsFlattened pins it.
status: addressed
---
