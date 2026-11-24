---
id: RR-M14N5W5
type: review-response
title: Message does not mention the cleanup migration
finding: The deprecation error could point to the dataentry-cleanup migration.
severity: minor
reason: Without a metamodel the migration removes only select, not multi-select, so pointing to it would be half true. The message names the exact line to remove.
status: wont-fix
---
