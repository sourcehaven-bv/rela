---
id: RR-EEFDVG
type: review-response
title: storedFacesOf treats a read error as 'deleted' (fail-open to the deleted-face rule)
finding: resolveHistorySubject split live-but-refused from deleted using storedFacesOf, which logs a read error and returns no faces. A store fault therefore routed a live face the caller cannot see to the deleted-face rule, which grants on the global history:read permission.
severity: critical
resolution: Added loadStoredFaces, which returns the error; storedFacesOf now wraps it for callers where 'nothing stored' leads to a refusal. resolveHistorySubject and authorizeRelationHistoryRead (both endpoints) use loadStoredFaces and route the error through writeGateError. Pinned by TestFacedHistory_StoredFacesReadErrorIsAnError.
status: addressed
---
