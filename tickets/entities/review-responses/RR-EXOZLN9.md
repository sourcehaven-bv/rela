---
id: RR-EXOZLN9
type: review-response
title: 'Code review: Revoke and Close race on one resident'
finding: tenantResident.Revoke and Close could both call dataentry.Retire concurrently; StopWatching is not synchronized.
severity: significant
resolution: tenantResident.retire (sync.Once) wraps Retire for both Revoke and Close. TestTenantMode_RevokeRacesLastRelease exercises the path on a real resident.
status: addressed
---
