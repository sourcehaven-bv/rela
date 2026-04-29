---
id: RR-1CE17
type: review-response
title: 'Test coverage gap: confirm-modal flow'
finding: No test asserts that triggerEl is forwarded (or not forwarded) through the confirmation flow. Given the bug above, this is the path most likely to regress silently. Add a test exercising triggerAction with action.confirm=true and verifying onRequestConfirm carries the right element through to executeAction.
severity: significant
resolution: Added test 'forwards triggerEl through onRequestConfirm for confirm-required actions' in useListActions.test.ts. Asserts onRequestConfirm receives the trigger element as the third argument when triggerAction is invoked with action.confirm=true.
status: addressed
---
