---
id: BUG-HOB9BR
type: bug
title: Relation-picker autosave aborts with "unknown types" when the linked entity is outside the 100-candidate window
description: |-
    Adding a second value to an existing legacy (non-cards) relation field on an edit form fails silently: autosave aborts with the toast "Some related entities have unknown types; relation changes were not saved. Reload the form and try again." and no PATCH request is sent at all. Reported as GitHub issue #1595 against a production Atlas instance (Projectmanager field on /form/edit_project/PROJ-USS5).

    The id -> type map (pickerTypes) that reshapeLegacyToModern needs is built exclusively from RelationPicker's `candidates` array, which loadCandidates fills with only the first 100 entities per target type (per_page: 100). Any already-linked entity outside that window has no resolvable type, so reshapeLegacyToModern returns null and buildAutoSaveRelationsBody aborts the WHOLE form's relations autosave — not just the affected field.
priority: high
effort: s
why1: reshapeLegacyToModern returned null because pickerTypes had no entry for an already-linked entity, so buildAutoSaveRelationsBody aborted the whole relations autosave and sent no PATCH.
why2: RelationPicker builds that id -> type map in buildOutgoingTypes() purely by scanning its local `candidates` array, so an ID absent from candidates simply has no type.
why3: 'loadCandidates() fills `candidates` with a single page — entitiesStore.fetchList(targetType, { per_page: 100 }) — so it only ever holds the first 100 entities per target type. Any linked entity beyond that boundary is invisible to the picker.'
why4: The picker treats a paged collection endpoint as if it returned the complete set. The identical mistake was already found and fixed for the kanban board (BUG-5OAQUG), which is why listAllEntities() exists in frontend/src/api/entities.ts — but that fix was applied to the one consumer that reported it, not to every consumer of the same endpoint.
why5: Nothing in the codebase makes single-page truncation visible at the call site. listEntities() returns a ListResponse whose meta.has_more says the set is incomplete, but ignoring it is silent and type-checks fine, so each new consumer re-makes the choice by accident. The safe helper (listAllEntities) is opt-in and undiscoverable from the unsafe one.
prevention: 'Two layers. (1) Make the truncation impossible to ignore at this call site by having RelationPicker page the collection like every other consumer that renders a complete set. (2) Make the general class visible: listEntities'' contract should point at listAllEntities, and any consumer that resolves IDs to entities client-side should be treated as a complete-set consumer by default. Longer term the honest fix is to stop deriving the type map from a candidate list at all — the server already knows each linked entity''s type and returns it on the entity read, so the picker should not be reconstructing it from a paged search.'
status: done
---
