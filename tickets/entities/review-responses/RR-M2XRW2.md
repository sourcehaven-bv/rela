---
id: RR-M2XRW2
type: review-response
title: Partial-presence where on a multi-type relation silently inverts a max gate
finding: 'The plan validated where properties at load only when target_type is set, leaving the unset case inconsistent. Worse than untidy: on a relation reaching taak and terugkerend where only one declares status, where: [''status!=gereed''] makes MatchAll error for every terugkerend target, and under max: an unevaluable target COUNTS AS MATCHING — so a max: 0 gate fires on entities that have only schedules. The ticket''s own motivating example walks up to this and did not name it.'
severity: significant
resolution: 'RelationDef.From/To are guaranteed non-empty (loader.go:562-569), so the reachable set is statically known even without target_type. Plan now specifies: validate where against the UNION of reachable types on the relevant side and fail at load when the property exists on NONE of them; emit a load-time WARNING naming the types when it is missing from SOME but not all, telling the operator to set target_type. This makes the unset path strictly better than today rather than merely unchanged.'
status: addressed
---
