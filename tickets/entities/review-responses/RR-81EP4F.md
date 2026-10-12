---
id: RR-81EP4F
type: review-response
title: Column Add prefill appended to template relations and was lost on template switch
finding: The per-column Add prefill was appended to the template's relation value instead of replacing it. Switching template dropped the prefill.
severity: minor
resolution: Fixed in 82f0f9a0d. For max_outgoing 1 the prefill replaces the template value and survives a template switch. Covered by the DynamicForm single-valued relation prefill tests.
status: addressed
---

Review finding R2-7.
