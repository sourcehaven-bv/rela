---
id: RR-DLD32N3
type: review-response
title: 'Code review: every provisioner command runs control DDL'
finding: suspend, resume and erase created rela_control on a fresh database, needed CREATE privilege, and shared the tenant lock class for the DDL lock.
severity: minor
resolution: 'Only Create runs ensureControl; other commands report ErrUnknownTenant without the table; the DDL lock uses its own class key. Test: TestProvisioner_OnlyCreateMakesTheControlTable.'
status: addressed
---
