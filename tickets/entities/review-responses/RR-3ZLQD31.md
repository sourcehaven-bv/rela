---
id: RR-3ZLQD31
type: review-response
title: Type assertion back channel for the writer accepts a typed nil
finding: lua.EntityManager is asserted to peersync.Writer; a typed nil *Manager passes and panics on first write.
severity: minor
resolution: isNilPointer rejects a typed nil. The assertion stays because Services is at its plimsoll method cap and the scheduler already asserts optional workspace capabilities; documented on peerDeps.
status: addressed
---
