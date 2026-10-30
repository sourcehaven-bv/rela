---
id: RR-B3F8XF8
type: review-response
title: loading is a synced copy of session.loading
finding: EntityDetail keeps a copy kept in sync by a watch instead of reading session.loading.
severity: nit
reason: useDelayedPending reads loading during setup before the session exists; a computed would fail there. Commented.
status: wont-fix
---
