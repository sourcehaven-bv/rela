---
id: RR-645BUH
type: review-response
title: Operator raw-store commands accept --run-as but their audit records drop asserted_by
finding: 'actsAsIdentity covers dev, migrate data/gc/adopt-face/baseline, so `rela --run-as system:ci dev seed --force` or `migrate data --apply` writes under system:ci, but the audit records at internal/cli/dev.go:91, internal/cli/migrate_data.go:558, internal/datamigration/run.go:219, gc.go:280 and adoptface.go:247 set Principal without AssertedBy. These are the sanctioned raw-store paths whose only accountability is their audit record (bulk seed, data migration, GC deletion), and the record names a self-asserted job identity with no trace of the OS user, against the ''asserted_by always recorded'' rule. Fix: add AssertedBy: audit.AssertedByFrom(ctx) at each site (pass ctx into the datamigration recorders), or refuse --run-as for these operator commands in commandIdentity. Pin it with a test that every audit.Record literal outside tests sets AssertedBy, so the next new Record site cannot silently drop it.'
severity: significant
resolution: 'Same fix as the first cr-t6b finding: AssertedBy at all five sites plus the AST guard.'
reason: 'Same fix as the first cr-t6b finding: AssertedBy at all five sites plus the AST guard.'
status: addressed
---
