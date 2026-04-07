---
id: RR-1HLF
type: review-response
title: 'Test gap: non-string task field values not covered'
finding: Only `task="yes"` is tested for non-bool task values. Should also test task=0, task={}, task=nil to lock in the truthy-bool semantics.
severity: significant
resolution: Added TestMdTaskListNonBoolTaskValues covering task="yes", task=1, task=0, task={}, task=nil, and task=false. Refactored renderer to use shared isTaskItem() helper that's explicit about the truthy-bool rule. All 6 cases produce a plain item.
status: addressed
---
