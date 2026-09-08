---
id: RR-FCUS1V
type: review-response
title: Entity rename silently orphans every comment on the renamed entity
finding: 'The plan keys comments by target entity ID but never mentions rename. store.EntityObserver (store.go:981) emits EXACTLY ONE callback on rename — EntityRenamed(oldID, renamed) — and explicitly NOT EntityDelete(oldID)+EntityPut. The godoc names the intended consumers: ''ID-keyed observers (waiver stores, anything that stores references by entity ID) can rewrite those references in one step.'' A comment store is exactly that. Without subscribing, every comment on a renamed entity becomes unreachable — silently, with no error and no orphan state, because the new ID simply has no comments. rela supports rename as a first-class operation (Manager.RenameEntity, manager.go:1165), so this is not a corner case. The plan must subscribe the comment store as an EntityObserver and handle EntityRenamed (re-key) as well as EntityDelete (AC9''s ''cleanup path'', currently undefined).'
severity: critical
resolution: 'Plan now has a Lifecycle section: the comment store subscribes as a store.EntityObserver and handles EntityRenamed(oldID, renamed) by re-keying the target''s comments, plus EntityDelete(id) by removing them. AC10 pins both.'
status: addressed
---
