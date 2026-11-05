---
id: RR-EXMLG83
type: review-response
title: Each joiner sends a redundant PATCH
finding: When the room base is newer than the client's GET, the joiner saves the room base body, which differs from its lastSeenContent, so a no-op PATCH goes out.
severity: minor
resolution: The join saves the room's document only when the room's base differs from the loaded body, and setContentBase moves the dedupe, so no redundant PATCH. A pending save of the body just stored is now also dropped (useAutoSave).
status: addressed
---
