---
id: RR-7T94EF
type: review-response
title: Embedded path's keeps-form-dirty guarantee is void; host unmounts the form immediately
finding: |-
    On upload failure in embedded mode (DynamicForm.vue:1124-1129) the code fires the error toast then emits 'inline-created'. But the consumers unmount the form as their first action: RelationPicker.vue:304 handleEntityCreated calls closeCreateModal() immediately, as does RelationCards.vue:755. So `dirty.value = true` protects nothing — there is no navigation guard left to consult, and the staged Files are discarded.

    Not silent (the toast fires), so significant rather than critical. But the comment at DynamicForm.vue:1116-1121 and the commit message both assert a property that is false on this path.

    Fix: either document the embedded exception explicitly, or do not emit 'inline-created' on failure so the host keeps the modal open and the user can act.
severity: significant
resolution: 'Resolved by the RR-4QO887 fix rather than separately: the failure path no longer sets dirty=true at all, so there is no longer a guarantee to be void on the embedded path. Staged files are cleared and the error toast fires before the inline-created emit, which is the part that actually informs the user. The misleading comment claiming the dirty flag protects this path was removed with the code it described.'
status: addressed
---

## Finding

The failure branch sets `dirty.value = true` "so the navigation guard still
warns", then the embedded path emits `inline-created`. Its consumers close the
modal as their first statement:

- `RelationPicker.vue:304` — `handleEntityCreated` → `closeCreateModal()`
- `RelationCards.vue:755` — same shape

Closing unmounts `InlineCreateFormModal` and the `DynamicForm` inside it, so
there is no guard left to consult and the staged `File` objects are discarded.

## Severity

Not silent — the error toast fires before the emit, so the user is told. But the
comment at `DynamicForm.vue:1116-1121` and the commit message both claim the
dirty flag protects this path, and on the embedded path it does not.

## Fix

Either state the embedded exception where the claim is made, or withhold the
`inline-created` emit on failure so the host keeps the modal open.
