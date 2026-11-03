---
id: RR-ZVKCJX
type: review-response
title: Docs overstate the one-problem rule
finding: 'docs-project/entities/guides/GUIDE-realtime.md:1744 - "The load reports the first such problem" reads as one problem per load. The literal policy ({{ and kind) reports every offending property at once, and when it reports any, the NormalizeProps rules, today() and when: are not checked at all. Say that the literal checks run first and the service-rule checks only after they pass, reporting the first refusal.'
severity: minor
resolution: "addressed\tThe guide now says the load reports a today() when: first, then every property that breaks the literal rules, and checks the service rules only when none does, reporting the first refusal; docs regenerated with just docs."
status: addressed
---
