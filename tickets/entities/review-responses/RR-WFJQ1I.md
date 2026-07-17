---
id: RR-WFJQ1I
type: review-response
title: Detail is per-violation (empty for Lua/then-filter), not per-entity; render must tolerate absent detail
finding: 'checkEntityAgainstRule emits one content Violation per entity, but a rule with lua: short-circuits Lua-first (validation.go:296-299) and Lua rules can emit multiple violations per entity (validation.go:337). So RuleResult.Violations can hold multiple entries with the same EntityID, and non-content (Lua/then-filter) violations carry Detail==nil. The plan''s mental model ''detail pairs with each entity'' should be ''detail pairs with each violation, empty for non-content''. Frontend must render nil/empty detail as ''no affordance'' (no empty tooltip).'
severity: minor
resolution: Plan step 2 + edge cases state Detail is per-violation and nil for Lua/then-filter violations; frontend renders absent/empty detail as no tooltip (issue.detail?.length guard).
status: addressed
---

See finding property.
