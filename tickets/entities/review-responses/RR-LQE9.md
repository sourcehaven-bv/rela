---
id: RR-LQE9
type: review-response
title: LintAll should return compiled programs to avoid double-parse
finding: 'lint.go: LintAll returns []Issue. A future ACL loader will want to (a) lint at load time, then (b) compile again to use. That''s a double-parse. Rename and reshape: `func CompileAll(env, sources []NamedSource) (programs []*Program, issues []Issue)` — returns nil entries in programs[i] where issues[i] applies. Removes the ''linted but compiled differently'' bug class. Cheap to do now.'
severity: minor
resolution: Renamed LintAll → CompileAll; signature returns (programs []*Program, issues []Issue) so a future ACL loader gets the compiled programs back without a double-parse. Tests renamed accordingly (TestCompileAll_ReportsAllSourceErrors, TestCompileAll_AllClean) and now assert that successful slots are non-nil and failure slots are nil.
status: addressed
---
