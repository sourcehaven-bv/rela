---
id: RR-ODGGH3
type: review-response
title: Guard test passes against an aliased raw-FS bypass
finding: TestRootedFS_ReachesFSOnlyThroughValidatedFS matched the literal r.fs.X with a regex. raw := r.fs followed by raw.ReadFile(bareString) — zero validation — passed it.
severity: critical
resolution: 'Rewritten over go/ast as TestRootedGo_NoRawFSOrDirectOSAccess: any SelectorExpr reading the fs field outside an exemption list and any os.* call fails. Mutation-verified against the exact alias; scope (rooted.go only) documented.'
status: addressed
---
