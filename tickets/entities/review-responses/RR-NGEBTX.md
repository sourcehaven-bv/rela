---
id: RR-NGEBTX
type: review-response
title: No upgrade test from a populated v6 database
finding: The shape test starts at v3 with no rows, so nothing asserted that pre-existing rows end up NULL or that re-opening is harmless.
severity: minor
resolution: 'Added sqlitedb TestMigrateToEditorColumns: seeds a v6 database with an entity and a relation, opens twice, asserts version 7, the columns exist, and existing rows are NULL.'
status: addressed
---
