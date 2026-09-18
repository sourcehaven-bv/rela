---
id: BUG-LSCDJK
type: bug
title: Relation edits silently dropped when a linked entity is outside the picker's first-100 candidate window
description: 'A relation picker resolves related-entity types only from its first-100-per-type candidate fetch. A pre-existing link whose target falls outside that window has no resolved type, so reshapeLegacyToModern returns null and the whole relations payload is discarded: autosave shows "Some related entities have unknown types; relation changes were not saved" and explicit Save aborts entirely.'
priority: high
effort: m
why1: On save, reshapeLegacyToModern returns null because pickerTypes has no type for a related entity id, so DynamicForm discards the relations payload (autosave) or aborts the save (explicit Save).
why2: RelationPicker builds its id->type map in buildOutgoingTypes purely from `candidates`, so any id not in that array gets no type.
why3: '`candidates` is a capped list read — entitiesStore.fetchList(targetType, {per_page: 100}) per target type — and it is a *candidate* list (what you could pick), so it is not guaranteed to contain the entity you have already picked. Past 100 entities of the target type, a pre-existing link falls outside it.'
why4: 'The picker had no source for a linked entity''s type other than that list: the entity GET serializes relations as map[string][]string (internal/dataentry/entityserializer.go:78), ids only. When TKT-GFQK/TKT-ZEKO4 moved the wire to JSON:API resource identifiers requiring `type`, the picker was made to derive the type from the candidate list rather than from the edges themselves, and no test covered a value id absent from candidates.'
why5: 'The widget derives a fact about *existing edges* from a list whose purpose is *offering choices*. The correct source was always available and is what makes the `cards` widget immune: the relations endpoint returns each edge with its `type`. The systemic cause is that the two widgets solve the same problem from different sources, and the picker''s source silently degrades with data volume, so the defect is invisible in small projects and in every test fixture.'
prevention: 'A widget must resolve facts about existing edges from the edges themselves, not from a paginated candidate/choice list. Concretely: any capped read (per_page, limit) must never be the sole source of truth for data outside that cap, and tests for list-backed widgets must include a fixture where the value lies outside the page — small fixtures make cap-dependent bugs unreproducible. Pinned here by RelationPicker.test.ts ''pre-existing value outside the candidate page (BUG-LSCDJK)''.'
status: done
---

On the data-entry edit form, changing a relation can fail with the toast "Some
related entities have unknown types; relation changes were not saved. Reload the
form and try again." Reloading does not help — the form is permanently unable to
save that relation.

**Expected:** a relation edit saves regardless of how many entities exist of the
relation's target type.

**Actual:** the relation part of the save is dropped (autosave) or the entire
save is aborted (explicit Save), with a toast telling the user to reload. The
toast's advice is wrong: the condition is deterministic, so reloading reproduces
it.

**Steps to reproduce:**

1. Have an entity type `T` that is the target of a relation `r`, with more
than 100 entities of type `T` in the project.
2. Link an entity `A` via `r` to a `T` entity that does not appear in the
first page (`per_page: 100`) of the type's list — e.g. one sorted late.
3. Open `A`'s edit form. The relation widget is a picker (not `cards`).
4. Change any relation on the form.
5. Observe the toast and the dropped relation write.

**Secondary symptom:** because `selectedEntities` also filters `candidates`, the
out-of-window link is not rendered as a selected chip at all, so the user cannot
see the link they already have.
