---
id: RR-KGY8N8
type: review-response
title: The base model path went through GITHUB_ENV
finding: A job-wide variable hides the dependency between the two steps.
severity: nit
resolution: The base step writes a step output that the next step reads.
status: addressed
---
