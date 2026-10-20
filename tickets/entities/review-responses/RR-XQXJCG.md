---
id: RR-XQXJCG
type: review-response
title: T7b is too large to review well
finding: '§12: T7b combines six library components with stories, the app panel, the EntityDetail mount, e2e and docs, and T7a ships composables with no consumer. Fix: three sequential PRs. T7a: backend descriptor and per-message affordances, the strict renderer, wire types. T7b: presentational library components and stories (pure, reviewable in Storybook). T7c: composables, ChannelsPanel, mount, e2e, docs. Each stays on develop, with at most one stacked.'
severity: minor
resolution: 'The work is split into three sequential PRs: T7a config, endpoint and plumbing; T7b pure library components; T7c panel, mount, e2e and docs (section 16).'
status: addressed
---
