---
id: RR-QQ1NGR
type: review-response
title: Parenthesized go literal recorded as closure
finding: go (func(){...})() missed the go-statement literal check.
severity: minor
resolution: walk unwraps ParenExpr before the FuncLit check; covered in TestFile_KeepsLineNumbersAndDirectives.
status: addressed
---
