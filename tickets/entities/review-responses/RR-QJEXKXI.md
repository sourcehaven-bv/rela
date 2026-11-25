---
id: RR-QJEXKXI
type: review-response
title: Boolean allow_acl_bypass rewritten to read+write silently at load
finding: '[security] acl-bypass-enum is a blocking per-file migration, so MigrateBytes applied it on every load: a boolean (or yes/on) allow_acl_bypass in schema.yaml became read+write with only a WARN line. The parser refuses that spelling on purpose so a person reviews the grant.'
severity: significant
resolution: Added migration.ReviewRequired; ACLBypassEnumMigration implements it and MigrateBytes refuses it with *migration.Error. Pinned by TestMigrateBytes_Refuses/privilege_rewrite_needs_review and the reload leak test.
status: addressed
---
