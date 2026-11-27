---
id: RR-V1H3S0V
type: review-response
title: Data-entry migrations see the migrated schema at boot but the raw schema in rela migrate
finding: MetamodelAware data-entry migrations (dataentry_cleanup, relation_direction) receive the loaded, already migrated metamodel at boot, while rela migrate passes LoadWithoutMigrationCheck output. When both files need migrating the results can differ in edge cases.
severity: minor
resolution: 'Superseded in code review (RR-XPOEID6): loadConfig passes no schema, as load-time detection always did, so in-memory migration applies exactly the set that used to refuse.'
status: addressed
---
