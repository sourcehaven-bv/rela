---
id: RR-C98K2V
type: review-response
title: Reporting regression covers all missing required properties, not only value problems
finding: 'Understanding item 3 says that "value problems are reported one at a time". Old literalProps also reported every missing required property in one pass, and still reported undeclared properties when max or when: failed. NormalizeProps stops at the first refusal of any kind, including the first missing required property in sorted order. State the full regression in item 3, so the acceptance covers what actually changes.'
severity: minor
resolution: "addressed\tUnderstanding item 3 now states the full regression: one NormalizeProps problem per load, including only the first missing required property and undeclared properties hidden by an earlier refusal."
status: addressed
---
