---
id: RR-DVPQIU
type: review-response
title: PatchEntity reports any read error as not found
finding: patchEntityOnce collapses every store error into not-found
severity: minor
resolution: Unchanged; pre-existing and documented at the call site
reason: The not-found here is structural for the Lua bindings and predates this change. CalDAV no longer infers hidden from it (finding 1) so the loss scenario is closed. Changing the error contract of PatchEntity for every consumer is out of scope for this ticket and would need its own audit of callers.
status: wont-fix
---
