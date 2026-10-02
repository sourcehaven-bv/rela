---
id: RR-OQJ5UF
type: review-response
title: Non-enum list columns drop every value after the first
finding: For list string/date/file/rrule properties the server widget is the scalar one, so values[0] dropped the rest.
severity: critical
resolution: 'A list: true property now passes the whole array; densePropertyRoutingHint routes a non-enum list to preformatted text via formatCellValue. Test: non-enum list shows all values joined; fails on develop.'
status: addressed
---
