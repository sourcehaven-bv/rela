---
id: RR-0O8JS
type: review-response
title: Dashboard createdIds shared mutable array across tests
finding: const createdIds at describe scope, push in beforeEach pop in afterEach. Each test has own backend so no real leak, but pattern is fragile. Use local arrays or a fixture that tracks creations.
severity: significant
reason: Each test has its own backend, so the shared array doesn't actually leak across tests. Pattern is idiomatic Playwright cleanup. A fixture-tracked auto-cleanup would be nicer but defer until it's actually needed.
status: deferred
---
