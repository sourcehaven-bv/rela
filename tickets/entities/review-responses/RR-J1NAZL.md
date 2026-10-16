---
id: RR-J1NAZL
type: review-response
title: 'Missing program for a collectable type silently drops its rows'
finding: 'applyCondition skipped a type with no program, so a reload between the view''s snapshot and the filter''s emptied the collection silently.'
severity: significant
resolution: 'A type in the step''s type set without a program now fails the view with errTraverseConditionUnavailable; only types the relation cannot hold are dropped. Test: TestExecuteViewWithCondition_StaleProgramFails.'
status: addressed
---
