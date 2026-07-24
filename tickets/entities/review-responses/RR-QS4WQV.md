---
id: RR-QS4WQV
type: review-response
title: No test constrains any production wiring site — all three revert to the raw store silently green
finding: 'Reviewer mutation-tested each production wiring site by substituting the raw store for the ACL-bound reader: dataentry/app.go:311, appbuild.luaReadDepsFor:234, appbuild cascade:819. ALL THREE pass every test. My reported ''raw store as reader → 4 fail'' mutation was applied to newACLWorld in aclreads_test.go — the test''s OWN fixture — which is self-referential: it proves the fixture wires what the test asserts, not that production wires anything. aclreads_test.go tests ScriptReader and the bindings well; it tests the WIRING not at all. For a security seam the wiring IS the control — a correct gate pointed at nothing is worth nothing. Needs at least one test per site running a script through the real App/Services under a restrictive policy.'
severity: critical
resolution: 'Added black-box wiring tests: internal/dataentry/luawiring_test.go (3 cases through the real App.luaWriteDeps) and internal/appbuild/luawiring_test.go (2 cases through Services.ScheduledLuaWriteDeps — which doubles as the role-scoped LLM-job proof deferred as AC12), plus internal/appbuild/scriptreader_internal_test.go pinning the shared helpers the cascade path uses. Mutation-verified: repointing dataentry''s VisibleReader at the raw store now fails 2 tests, the tracer mutation fails 1, and the appbuild scheduler mutation fails 2 — all previously silent.'
status: addressed
---
