---
id: RR-ES596P
type: review-response
title: Stacked PRs share the locale files
finding: PR 2 adds keys to the same en.json and nl.json and lowers the same allowlist totals as PR 1, so a squash merge of PR 1 forces a rebase of PR 2.
severity: minor
resolution: PR 2 is stacked on PR 1 and rebased after PR 1 merges; keys are grouped by area so conflicts stay mechanical.
status: addressed
---
