---
id: RR-M292PZ
type: review-response
title: Playwright worker comment overclaimed isolation; postgres container and CPU are shared
finding: 'The comment added beside `workers: 4` asserted that ''every shared resource is already per-worker''. That is stronger than what the code provides. Per-test state is isolated (ephemeral port, mkdtemp dir, pid-keyed schema), but the postgres:16 service is a single container every worker''s server pools against, and the runner''s 4 cores now carry roughly 8 CPU-hungry processes (Chromium plus rela-server per worker) against 5s action and 10s navigation timeouts. With retries: 2, load-induced flake passes on retry and reports green, so the cost surfaces as wall-clock regression rather than a visible failure. A future reader would have built on a safety property that was never real.'
severity: minor
resolution: The reviewer's concern about the comment overclaiming was valid and the comment is rewritten. The worker count is reverted to 2, but NOT for the reason first recorded here. Raising it to 4 was blamed for an E2E failure in document-edit-button.spec.ts; that attribution was wrong — the same spec failed again at 2 workers, and the real cause was the branch being 7 commits behind develop. After rebasing, run 35337622282 was fully green. The revert stands anyway because the change was never independently justified once its measured benefit (E2E 409s to 361s) is set against having caused a false diagnosis, and because TKT-LP4EE8 (the unbounded per-request cmdexec runner in internal/dataentry, genuinely found along the way) is the right prerequisite for revisiting it. Raising workers is a tier-2 item gated on that ticket.
status: addressed
---
