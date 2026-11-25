---
id: RR-6FNKJB7
type: review-response
title: '[security] A guard can disclose one bit of a hidden field'
finding: '[security] A change or predicate guard that reads a field the author cannot see tells the author through the move whether it holds.'
severity: minor
resolution: The flows guide tells authors to write change and predicate guards over fields the editors can read.
status: addressed
---
