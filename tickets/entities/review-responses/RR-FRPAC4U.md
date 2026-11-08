---
id: RR-FRPAC4U
type: review-response
title: Docs overstate the erasure and lock guarantees
finding: The guide claimed the erase is always refused while a server has the tenant open, omitted PgBouncer transaction mode and lockless access, and claimed all data was removed.
severity: significant
resolution: 'Rewrote the section: exact guarantee, what is not removed (other hosts, logs, backups), lock lifetime, session-pooling requirement, lockless access.'
status: addressed
---
