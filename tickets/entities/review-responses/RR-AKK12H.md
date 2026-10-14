---
id: RR-AKK12H
type: review-response
title: Markdown refs accepted in append_section
finding: Parse accepted {{item.summary}} in append content; a flattened converted heading forges a sibling section. Docs said it was refused.
severity: significant
resolution: validateRefs rejects markdown refs in any append_section content; test added.
status: addressed
---
