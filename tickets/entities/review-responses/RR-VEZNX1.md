---
id: RR-VEZNX1
type: review-response
title: False boolean table cells render as Yes
finding: Dense routing sends booleans to formatCellValue, which tested truthiness; the server's string false is truthy, so every boolean cell showed Yes.
severity: critical
resolution: 'formatCellValue reads a string boolean as the value it spells. Tests: table cell true shows Yes and false shows No; format.test.ts covers the string forms.'
status: addressed
---
