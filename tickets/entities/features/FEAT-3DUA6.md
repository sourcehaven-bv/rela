---
id: FEAT-3DUA6
type: feature
title: 'analyze unique: report pre-existing unique-constraint violations'
summary: 'New `rela analyze unique` command + `analyze_unique` MCP tool that report same-type entities sharing a value for a `unique: true` property.'
description: 'Read-side companion to the write-path unique constraint (FEAT-OF2ZOL). The write path rejects NEW duplicates on a `unique: true` property, but enabling the flag does not retroactively clean data that already contains collisions. This adds analysis.Service.FindUniqueViolations, surfaced as `rela analyze unique` (text + JSON), folded into `rela analyze all` (summary count + section), and as the `analyze_unique` MCP tool. List properties are skipped and empty values are exempt, matching the write-path check. Enables an operator to find and fix pre-existing duplicates before/after adding `unique: true`.'
priority: low
status: proposed
---
