---
id: RR-9MJRJG
type: review-response
title: RelationFilterDirection resolves nondeterministically over map iteration
finding: 'Config.Lists is map[string]List and RelationFilterDirection iterates `for _, list := range c.Lists` (config.go:278). Go randomizes map iteration, so when two lists of the same entity type filter the same relation with opposite valid directions, the winner varies per call/process — the same list request can traverse incoming on one process and outgoing on another. The doc comment claims deterministic first-match-wins (config.go:270-273), which is false for a map range. CollectConfigWarnings iterates sortedListIDs (deterministic) but only warns on the wrong-`to`-side case, not on two legitimately-valid conflicting-direction lists, so this passes validation then resolves randomly. Fix: sort list IDs here and document lowest-ID-wins, OR make conflicting directions a hard ValidateConfig error since the shared pipeline can''t honor both. Low probability, real correctness bug. Untested.'
severity: significant
resolution: 'RelationFilterDirection now iterates lists via sortedListIDs (deterministic lowest-list-ID-wins). CollectConfigWarnings gained conflictingRelationDirectionWarnings, flagging two lists of a type that configure the same relation with opposite directions. Tests: TestCollectConfigWarnings_ConflictingRelationDirections. Commit 72f10b99.'
status: addressed
---
