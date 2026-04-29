---
id: RR-7HDS3
type: review-response
title: Confirm-modal flow drops the trigger element
finding: 'When an action has confirm: true, EntityList.vue:689 calls executeAction(pendingAction.id, pendingAction.config) from @confirm with no third argument. A confirmed action that throws a ScriptError opens the dialog with triggerEl = null and focus restoration falls to document.body. The plumbing exists for keyboard accessibility, and the confirm path is the one users with screen readers are most likely to traverse. Fix: either widen pendingAction to {id, config, triggerEl} or capture the modal''s confirm button as the focus target.'
severity: significant
resolution: Widened pendingAction in EntityList.vue to {id, config, triggerEl}. Updated useListActions onRequestConfirm signature to (action, actionId, triggerEl). triggerAction passes triggerEl to onRequestConfirm. The @confirm handler now forwards pendingAction.triggerEl to executeAction. Confirmed-action script errors now restore focus to the original trigger button.
status: addressed
---
