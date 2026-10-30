---
id: RR-NPPKQMR
type: review-response
title: Width counts bytes
finding: len counts bytes so non-ASCII text wraps early.
severity: minor
reason: Only the layout changes and splitting stays UTF-8 safe. Documented on foldWidth.
status: wont-fix
---
