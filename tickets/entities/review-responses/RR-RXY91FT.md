---
id: RR-RXY91FT
type: review-response
title: 'why: group semantics differ between Go and TS'
finding: Go exempted the whole group and TS judged each line.
severity: significant
resolution: 'Both judge the adjacent comment group: the first line must start with why: and give a reason. Tests added.'
status: addressed
---

Go exempted the whole group and TS judged each line.
