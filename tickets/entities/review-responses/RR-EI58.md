---
id: RR-EI58
type: review-response
title: TestConcurrentReloadDuringRead does not exercise concurrent writers
finding: 'The new test only has reader goroutines and a reloader goroutine. The real races live between Reload and CreateEntity/UpdateEntity/DeleteEntity/indexEntity/removeFromIndex. Without a writer goroutine the test cannot catch the torn-graph or stale-index bugs. Also: 100ms is marginal for race detection, readers=8 is low, and the only correctness assertion is Meta() != nil which is trivially true.'
severity: significant
resolution: 'Rewritten as TestConcurrentReloadStateSnapshot. Changes: (1) reader count scales with GOMAXPROCS (minimum 4); (2) duration bumped to 300ms; (3) added a writer goroutine calling CreateEntity; (4) added an external writeMu serializing writer and reloader (mirroring the production App.writeMu discipline); (5) readers assert Meta().GetEntityDef() returns ok on every iteration, which fails if a torn or nil state is observed; (6) test documentation explicitly calls out the pre-existing graph-torn and entity-map races that are out of scope. Passes cleanly under -race after the workspaceState bundling.'
status: addressed
---
