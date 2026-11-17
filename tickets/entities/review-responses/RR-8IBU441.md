---
id: RR-8IBU441
type: review-response
title: Parse returns one multi-line error
finding: triggers.Parse joins its problems into one error string, so rela validate prints them on a single line.
severity: nit
reason: 'Cosmetic: the problems are already one per line inside the message. A typed problems error touches every caller of triggers.Parse; better done with the Configure trigger editor in TKT-ZT2KREN which needs per-problem output.'
status: deferred
---
