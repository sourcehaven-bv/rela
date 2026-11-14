---
id: RR-5536IEQ
type: review-response
title: rela flow validate stops at the first broken source
finding: A schema transitions error returned before flows.yaml was checked.
severity: minor
resolution: validate now checks both sources and joins their errors.
status: addressed
---
