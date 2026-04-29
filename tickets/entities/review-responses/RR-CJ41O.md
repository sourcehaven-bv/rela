---
id: RR-CJ41O
type: review-response
title: Cancelled fetches counted as failures
finding: If a list-action is cancelled (route navigation mid-action), the rejection is a CanceledError. isScriptError correctly returns false so the dialog stays closed, but the toast still says '1 failed' which is misleading. Pre-existing behaviour but worth a TODO or follow-up ticket.
severity: minor
resolution: Pre-existing behaviour, not introduced by this branch. Cancelled-fetch handling spans more than just useListActions and warrants its own ticket. Tracked as a follow-up rather than blocking this v1 ship.
reason: Pre-existing behaviour, not introduced by this branch. Cancelled-fetch handling spans more than just useListActions and warrants its own ticket. Tracked as a follow-up rather than blocking this v1 ship.
status: deferred
---
