---
id: RR-26YRHK
type: review-response
title: '[security] Access log kept numeric values under any query key'
finding: '[security] queryValueShape kept any numeric value, so ?pin=4821 or ?phone=0612345678 logged the number.'
severity: minor
resolution: Numbers are now kept only under paging keys (page, per_page, limit, offset, depth, scope_page); TestQueryShape covers pin, phone and relation=42.
status: addressed
---
