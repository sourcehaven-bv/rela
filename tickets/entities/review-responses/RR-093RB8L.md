---
id: RR-093RB8L
type: review-response
title: 'Design: long streams keep a revoked tenant''s lock'
finding: Revalidate marks a referenced resident evicting; an SSE or co-editing stream holds the lease and the shared lock, so erase is refused indefinitely.
severity: significant
resolution: 'Registry.revoke calls Revoker.Revoke (dataentry.Retire) on an in-use resident to end its streams; the resident stays in the map flagged revoked and closes on its last release. Acquire returns ErrRevoking (503) meanwhile. Tests: TestRegistry_RevokedInUseResident, TestRegistry_RefusedOrgIsClosed.'
status: addressed
---
