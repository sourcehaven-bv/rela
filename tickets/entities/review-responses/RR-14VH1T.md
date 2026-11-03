---
id: RR-14VH1T
type: review-response
title: Pick one clock mechanism for the grace-period test
finding: The plan leaves open whether to add a clock field to FSStore or set mtimes with os.Chtimes. Chtimes is enough and keeps a test-only field off a type that plimsoll already limits. Choose Chtimes in the plan and drop the clock field option.
severity: nit
resolution: 'Plan updated: the grace-period tests use os.Chtimes; no clock field.'
status: addressed
---
