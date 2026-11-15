---
id: RR-RQ2INPS
type: review-response
title: Erased tenant resurrected by lazy provisioning
finding: Erase deleted the control row, so an erased org looked absent. Lazy provisioning or a replayed organization.created would then create the tenant again.
severity: significant
resolution: 'Fixed in f71a306bb: erase keeps a tombstone (status erased, erased_at, no data). The resolver reports ErrTenantErased, never ErrTenantAbsent, so the host refuses it with 403 and acknowledges the webhook. Provisioner.Create refuses an erased tenant unless tenant create -recreate is used. Pinned by tombstone_postgres_test.go.'
status: addressed
---
