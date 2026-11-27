---
id: RR-3GYR0T4
type: review-response
title: '[security] on: predicate with per_subject many starts unbounded runs'
finding: '[security] A predicate start with per_subject: many starts another run on each edit that makes when: true again, which an editor can repeat without limit.'
severity: significant
resolution: 'flowcfg rejects on: predicate with per_subject: many, as it does for schedule; a load test pins it.'
status: addressed
---
