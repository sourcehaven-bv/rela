---
id: RR-N0VWDPO
type: review-response
title: Derived indexes read un-migrated data-entry.yaml
finding: staticIndexSpecs and rela db reconcile pass raw bytes to queryplan.LoadStaticIndexSpecs, which fails on deprecated keys, so a booted postgres/sqlite server skips index reconciliation.
severity: significant
resolution: LoadStaticIndexSpecs runs the same MigrateBytes call as loadConfig (arch rule for queryplan allows migration, with a reason). Pinned by TestLoadStaticIndexSpecsMigratesDeprecatedSyntax, which fails without the fix.
status: addressed
---
