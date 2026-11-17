---
id: RR-I92MF14
type: review-response
title: A lowered or/not must never widen to the whole type
finding: LowerCondition keeps Props/Traversals only for a plain conjunction. A caller that copies only those fields (appbuild lowerQueryCondition) would push an empty narrowing for a or b and serve every row. Use one representation that forces every caller to handle Where.
severity: critical
resolution: LowerCondition returns one ConditionLowering with Props, Traversals and Where. QueryConditions.Lower returns a visibility.PageQuery and every caller (mcp ofType, listpushdown narrowed, appbuild) sets Where; queryconditions_test covers or and not lowering.
status: addressed
---
