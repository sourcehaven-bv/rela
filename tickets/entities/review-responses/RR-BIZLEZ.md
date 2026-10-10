---
id: RR-BIZLEZ
type: review-response
title: Field verdict not re-checked after the slow upload
finding: '[security] The fields: verdict is taken in the preflight and not re-checked under the attachment lock. A when:-conditional rule can flip during a long upload and scan.'
severity: minor
reason: 'Deferred: PATCH and the ACL check have the same check-then-write shape. Closing it needs a field check in attachment.Deps under the lock, a change to the shared attachment service beyond this bug. The window only matters for a conditional read-only rule.'
status: deferred
---
