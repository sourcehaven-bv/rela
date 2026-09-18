---
id: RR-M292PZ
type: review-response
title: Playwright worker comment overclaimed isolation; postgres container and CPU are shared
finding: 'The comment added beside `workers: 4` asserted that ''every shared resource is already per-worker''. That is stronger than what the code provides. Per-test state is isolated (ephemeral port, mkdtemp dir, pid-keyed schema), but the postgres:16 service is a single container every worker''s server pools against, and the runner''s 4 cores now carry roughly 8 CPU-hungry processes (Chromium plus rela-server per worker) against 5s action and 10s navigation timeouts. With retries: 2, load-induced flake passes on retry and reports green, so the cost surfaces as wall-clock regression rather than a visible failure. A future reader would have built on a safety property that was never real.'
severity: minor
resolution: 'Rewrote the comment to state plainly what is isolated and what is not, and to name retries: 2 as the thing that will mask load flake as a slow green. The worker count stays at 4 — defensible on a 4-core runner and it produced a measured E2E improvement (409s to 361s) — but the comment no longer overstates why.'
status: addressed
---
