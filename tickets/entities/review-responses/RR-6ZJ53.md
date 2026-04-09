---
id: RR-6ZJ53
type: review-response
title: No integration test covers EntityList modal wiring (onDelete → pendingDelete → modal → API)
finding: The four-step chain onDelete → pendingDelete assignment → ConfirmModal render → confirmDelete → entitiesStore.remove has no end-to-end test. useListKeyboard.test.ts verifies the callback fires in isolation, ConfirmModal.test.ts verifies the modal renders in isolation, but nothing verifies they are wired correctly in EntityList.vue. Add an integration-style test that mounts EntityList, dispatches Delete keydown, asserts modal renders with the selected entity, clicks confirm, and asserts the API call.
severity: significant
resolution: 'Added frontend/src/components/lists/EntityList.test.ts with 7 integration tests covering: (1) modal is not shown by default, (2) clicking delete button opens modal for that entity, (3) Delete keydown on selected row opens modal for that row, (4) Backspace on selected row opens modal, (5) Cancel closes modal without calling remove, (6) Confirm calls entitiesStore.remove with the correct entity, (7) on error the modal stays open and busy state clears. The tests seed schemaStore.lists and entityTypes, stub entitiesStore.fetchList/remove, and mount EntityList with attachTo: document.body so Teleported ConfirmModal markup is visible to queries.'
status: addressed
---
