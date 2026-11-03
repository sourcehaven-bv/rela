---
id: RR-MV933E
type: review-response
title: asserted_by missing from CLI-reachable audit records
finding: "The diff adds AssertedBy to about 30 audit.Record literals by hand but misses these ones that run under --run-as: internal/cli/dev.go:91 (OpPerfSeed), internal/cli/migrate_data.go:558 (baseline), internal/datamigration/run.go:219 (OpDataMigration), internal/datamigration/gc.go:280 (OpDataGC), internal/datamigration/adoptface.go:247 (OpDataAdoptFace). docs/acl-security.md:1081 and docs/cli-reference.md say asserted_by is recorded in every graph audit record of the command.\t`RELA_RUN_AS=system:ci rela migrate data --apply` (or `rela dev seed`) writes an audit record with principal system:ci and no asserted_by. A raw-store bulk write by a self-asserted identity then cannot be traced to the OS user, which is the case the marker exists for.\tAdd AssertedBy to these sites. Better, stop stamping by hand: add a helper such as audit.Attribution(ctx) that returns Principal, TriggeredBy and AssertedBy in one value, and use it in every literal. Pin it with an AST test that fails on any audit.Record literal that sets Principal but not AssertedBy, so the next new op cannot forget it."
severity: significant
resolution: Added AssertedBy at dev seed, migrate baseline, datamigration run/gc/adopt-face, fsimport, db load markdown import and configedit (ctx passed into the recorders). audit.AssertedByFrom stays the one helper. New archguard test TestAuditRecordsCarryAssertedBy fails on any audit.Record literal in non-test code that sets Principal without AssertedBy; allowlist is empty.
reason: Added AssertedBy at dev seed, migrate baseline, datamigration run/gc/adopt-face, fsimport, db load markdown import and configedit (ctx passed into the recorders). audit.AssertedByFrom stays the one helper. New archguard test TestAuditRecordsCarryAssertedBy fails on any audit.Record literal in non-test code that sets Principal without AssertedBy; allowlist is empty.
status: addressed
---
