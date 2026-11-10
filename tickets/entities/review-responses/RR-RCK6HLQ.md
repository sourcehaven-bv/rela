---
id: RR-RCK6HLQ
type: review-response
title: 'CI: tenant lock still held right after close'
finding: 'TestErase_RemovesOnlyThatTenant failed in the Postgres Backend job: erase found the tenant locked right after the registry closed it. Closing a pgx connection does not wait for the server to end the session, so its advisory locks are released later.'
severity: significant
resolution: pinnedConn.Close unlocks the shared lock before closing, and the create and erase connections run pg_advisory_unlock_all before closing (closeReleasing).
status: addressed
---
