---
id: RR-J1CFYRR
type: review-response
title: 'Design: status read before the lock'
finding: Resolve reads active before pinSchema takes the shared lock; a status change between them still opens the tenant.
severity: significant
resolution: 'Registry.openPending resolves the tenant again after the open holds the schema lock and closes the store when it is refused. Test: TestRegistry_StatusChangeDuringOpen.'
status: addressed
---
