---
id: RR-Z7HYW7
type: review-response
title: Delete-failure test didn't assert optimistic-remove/rollback/toast
finding: The rewritten failure test only checked deleteEntity was called and the modal closed — the headline optimistic-remove + rollback + toast behavior had no component-level assertion; dropping the onError handler would have kept it green.
severity: significant
resolution: Strengthened the test to assert the row is restored after the rejected mutation settles (rollback) and uiStore.error fired (toast). The fleeting synchronous optimistic-removed frame was deliberately not asserted (non-deterministic to observe); the rollback+toast assertions still pin the onError wiring. Split modal-close into its own case.
status: addressed
---
