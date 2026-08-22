---
id: RR-DU4BUS
type: review-response
title: Migration/GC deletes and renames bypass synchronous version capture on pg
finding: The plan routes migration steps and GC through raw store writes, but on postgres, delete and rename version capture happens SYNCHRONOUSLY at the entitymanager boundary (they carry pre-delete state / old→new id the sweep cannot reconstruct — see CLAUDE.md versioning section and internal/entitymanager/version_hook.go). Raw drop_property rewrites are fine (sweep captures updates), but raw drop_entities/drop_relations/rename_entity_type lose the pre-delete/pre-rename snapshot entirely — including cascade-deleted relations (DeleteResult.DeletedRelations is consumed at the entitymanager boundary). This breaks the plan's own recoverability claim ('recoverable from entity_versions') exactly for the destructive operations that need it most.
severity: significant
resolution: 'Amendment A1 in TKT-0C57FS: migration runner and GC engine perform their own synchronous version capture for drop_entities/drop_relations/rename_entity_type by type-asserting the optional VersionWriter/RelationVersionWriter capabilities (mirroring entitymanager''s version_hook), including cascade-deleted relations via DeleteResult.DeletedRelations. No-op on stores without the capability (fs/mem — git is the recovery path there). AC9/AC11 updated to test it.'
status: addressed
---
