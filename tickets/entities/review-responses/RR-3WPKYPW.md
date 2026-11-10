---
id: RR-3WPKYPW
type: review-response
title: 'Design: audit sink ownership unclear'
finding: The sink must reach buildEntityManager, cascadeScriptRunner and data migration via cfg.Audit. Services.Close must not close a sink it does not own on the Discover path.
severity: significant
resolution: OpenAndAssemble takes the sink as a required argument; Services owns and closes it. SharedBaseOpener gives each tenant <cache>/audit/tenants/<schema>. Pinned by TestTenantIsolation_OwnAuditAndScriptEngine and TestFilesystem_CloseDropsLaterRecords.
status: addressed
---
