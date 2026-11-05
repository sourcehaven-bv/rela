---
id: RR-9LYVP2G
type: review-response
title: Parser and gate classified entity versus relation differently
finding: '[security] conflict.isRelationFile split the whole path on --, while the gate decided entity versus relation from the directory. A -- in a directory name made the parser read an entity file as a relation, so the detail judged it on the relation branch and served values that visible: hides.'
severity: significant
resolution: isRelationFile looks at the file name only. identifyConflict and identifyRelationConflict treat a side whose parsed kind disagrees with the directory as unjudgeable (uniform 404). Pinned by TestV1Conflicts_KindFollowsDirectory, TestIdentifyConflict_SideKindMustMatchDirectory and new TestIsRelationFile rows.
status: addressed
---
