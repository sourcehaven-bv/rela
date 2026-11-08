---
id: RR-4M9O6BN
type: review-response
title: Misleading collision comment and in-use error
finding: Both sides try the lock, so nothing waits; the in-use error was also returned during another erasure.
severity: nit
resolution: 'Comment corrected; ErrTenantInUse now reads "locked: open in a running server or being erased".'
status: addressed
---
