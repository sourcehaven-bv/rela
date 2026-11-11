---
id: RR-2P9AYKM
type: review-response
title: Relation conflict rules accepted where they never apply
finding: relations conflict and field_conflicts were accepted on pull-only remotes and on the content field.
severity: minor
resolution: Compile refuses them (relationRules). TestCompile_RelationRulesNeedPushedMeta pins it.
status: addressed
---
