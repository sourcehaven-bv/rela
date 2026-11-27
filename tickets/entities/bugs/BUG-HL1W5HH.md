---
id: BUG-HL1W5HH
type: bug
title: Relation trigger writes skip relation rules and strict validation
description: 'A set written by a relation_created or relation_removed trigger skipped write-enforced relation rules, strict validation mode and decimal-as-text storage that PatchEntity applies (issue #2636).'
priority: high
effort: s
why1: writeRelationTriggerSet called writeChecked with a nil txCheck and applied only structural validation errors.
why2: The trigger write path was written as a separate copy of the update path instead of sharing its check list.
why3: 'PR #2623 compared the new path with PatchEntity by reading, and missed relation rules, the validation mode and decimal storage.'
why4: No test drove a trigger write against a rule or mode that PatchEntity refuses.
why5: Nothing ties the checks of the two write paths together, so a check added to one is not added to the other.
prevention: relation_trigger_checks_test.go runs each refusal through both PatchEntity and a relation trigger, and the godoc lists the checks the trigger path runs and those it skips on purpose.
started: "2026-11-27"
completed: "2026-11-27"
status: done
---
