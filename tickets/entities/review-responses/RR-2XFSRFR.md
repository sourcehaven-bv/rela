---
id: RR-2XFSRFR
type: review-response
title: Strict schema grows with every row
finding: One schema entry per row id makes the strict schema large at max rows.
severity: significant
resolution: 'Plan: rows sharing a writable set share one $defs entry; a test checks the schema size at max.'
status: addressed
---
