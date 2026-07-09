---
id: RR-LH9RJ8
type: review-response
title: restoreRecreate bypasses per-field write ACL — resurrection can materialize forbidden field values
finding: 'cranky-code-reviewer S3: restoreRecreate (history_restore.go:114-135) applies the snapshot via CreateEntity, which runs type-level ACL + validation but NOT the per-field affordance gate (validateFieldWrite). So a principal with create rights on a type but no write access to field X can restore a deleted entity and bring X back to its historical value — laundering a forbidden field write through resurrection. RR-VOYXRV fixed this for the update path (restoreOntoLive) but the recreate path has the same gap. Fix: run the recreate''s property set through validateFieldWrite against a synthesized target (every snapshot property treated as a set), OR document + sign off that resurrection intentionally ignores field-write ACL.'
severity: significant
resolution: 'Fixed: restoreRecreate now runs every snapshot property through affordanceService.validateFieldWrite against the synthesized target before CreateEntity, so a field the principal can''t write blocks the resurrection — closing the field-write-ACL launder. Matches restoreOntoLive''s gate.'
status: addressed
---
