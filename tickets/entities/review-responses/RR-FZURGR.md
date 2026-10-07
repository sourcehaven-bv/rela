---
id: RR-FZURGR
type: review-response
title: Wrong .rela/ config allowlist
finding: config.yaml holds formatting; mail is mail.yaml and AI is ai.yaml, which the plan did not copy.
severity: significant
resolution: 'Plan updated: Every .rela/ entry is classified as config file, state key or skipped; unclassified entries are reported.'
status: addressed
---
