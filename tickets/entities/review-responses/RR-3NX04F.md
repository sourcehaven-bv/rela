---
id: RR-3NX04F
type: review-response
title: Escape during preflight does not cancel the conversion
finding: close() has no busy guard; when the preflight returns with no required fields, commit() runs and creates the entity the user dismissed.
severity: minor
resolution: Closing the dialog sets a cancelled flag; after the preflight the flow stops and clears the busy mark. Pinned by a component test.
status: addressed
---
