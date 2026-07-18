---
id: RR-NI145G
type: review-response
title: Stale _transitions menu after in-form status commit
finding: '_transitions is loaded once in DynamicForm.loadEntity into transitions.value. A move commits via updateField -> autoSave.scheduleFieldSave. The PATCH response carries fresh _transitions, but useAutoSave.mergeServerResponse only consumes entity.properties/content — it never refreshes _transitions (nor _fields/_relations), and there is no post-save loadEntity. So after picking a move, formData.status updates but transitions.value still holds the PREVIOUS state''s edges: StatusControl filters to!==modelValue and offers moves that are not real edges from the new state while hiding the genuine ones. The codebase already has the fix pattern: onAttachmentChanged does `await loadEntity(true)` to refresh _attachments. A machine-field commit needs the same reload (or extend mergeServerResponse to re-emit _transitions).'
severity: significant
resolution: DynamicForm's autoSave applyServerProperty callback now reloads the entity (loadEntity(true), fire-and-forget) when the applied property is a machine field (present in transitions.value), refreshing _transitions to the post-move state. Mirrors onAttachmentChanged's reload for _attachments.
status: addressed
---
