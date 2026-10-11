---
id: RR-WXMQ7Q
type: review-response
title: Visibility should not compute titles
finding: Putting DisplayTitle in internal/visibility mixes presentation into the gating package.
severity: minor
resolution: 'Visibility only gates and redacts. Titles are computed at each surface: buildMention in dataentry, svc.Meta.DisplayTitle in the CLI, convertStoreEntitySummary in MCP, EntityToTable in Lua.'
status: addressed
---
