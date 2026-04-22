---
id: RR-B8GJT
type: review-response
title: findFreePort TOCTOU with parallel workers
finding: server.close() releases the port, then hand it to spawn, and ~ms later the Go binary calls bind(2). With fullyParallel and multiple workers, kernel may allocate the same ephemeral port to another process. Real flake risk at ~1-in-500.
severity: critical
resolution: Added spawnServer with retry loop (3 attempts) — each attempt calls findFreePort fresh and waits for a /api/v1/_config probe. Retries catch TOCTOU reassignment. See e2e/tests/fixtures.ts spawnServer.
status: addressed
---
