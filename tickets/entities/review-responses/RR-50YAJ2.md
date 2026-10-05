---
id: RR-50YAJ2
type: review-response
title: A deleted face of a live entity skipped the entity row gate
finding: When sibling faces still live, a named deleted face fell through to the deleted-face rule (history:read plus the face grant) without evaluating the entity's row gate on its live faces.
severity: significant
resolution: In that arm resolveHistorySubject now requires vr.family(type, id) to report a readable face; a gate error goes to writeGateError, not-readable is the uniform 404. Pinned by TestFacedHistory_DeletedFaceGates/no_live_face_readable. Documented in acl-security.
status: addressed
---
