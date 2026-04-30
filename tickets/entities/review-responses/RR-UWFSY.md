---
id: RR-UWFSY
type: review-response
title: onConfirm error-handling boilerplate is duplicated across every call site
finding: 'Three independent contracts must be remembered at every call site: surface-the-error toast, rethrow inside onConfirm, swallow at the modal boundary. EntityDetail and EntityList both hand-roll the try/catch/toast/throw pattern; App.vue plus three test harnesses all .catch(()=>{}) at the boundary. Extract a helper: function withConfirmError<T>(action: () => Promise<T>, errMsg: string): () => Promise<void> that toasts and rethrows. One source of truth.'
severity: significant
resolution: Added withConfirmError(action, errMsg, uiStore) helper in useConfirm.ts that toasts and rethrows. EntityList.requestDelete and EntityDetail.requestDelete both use it now. App.vue retains its handleConfirm wrapper for the modal-boundary catch. Test harnesses still need the .catch(()=>{}) at the boundary, but the application code is one-line per call site. Tested in useConfirm.test.ts.
status: addressed
---
