---
id: RR-00EJVZ
type: review-response
title: Regression test covers only one of the three file types rela migrate scans
finding: internal/projectsetup/migrate.go:259-282 (getMigrateFiles) scans schema.yaml, data-entry.yaml and acl.yaml. The new test hardcodes migration.FileTypeMetamodel against result.SchemaPath only. That is sufficient today because Initialize writes only schema.yaml, but the test's stated purpose is to catch FUTURE rollouts that forget the init template — so if Initialize ever grows a starter data-entry.yaml or acl.yaml, the guard keeps passing while the new file rots. The forward-looking guard has a hole in exactly the dimension it claims to protect.
severity: significant
status: open
---
