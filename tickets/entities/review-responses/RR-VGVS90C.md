---
id: RR-VGVS90C
type: review-response
title: Type-less automations compile clauses for every type at load
finding: 'A type-less automation compiles its when and validate clauses for every declared type up front: automations x types x clauses programs.'
severity: minor
resolution: Documented on compileTypes. The cost is paid once at load, in place of a compile on the first write of each type.
status: addressed
---
