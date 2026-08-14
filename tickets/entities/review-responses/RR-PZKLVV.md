---
id: RR-PZKLVV
type: review-response
title: E2E ACL proof layer assumed a multi-principal fixture that does not exist
finding: |-
    The plan's Test Plan states the E2E layer is "the only layer that proves the SPA actually consumes the new endpoint" and proposes extending e2e/tests/dashboard.spec.ts to assert a gated card is absent for a non-holder. But the E2E suite has no multi-principal / acl.yaml fixture. The only ACL-adjacent E2E is e2e/tests/read-only-mode.spec.ts, which spawns its own server for a PROCESS FLAG (--read-only, line 91) — not a per-principal policy. There is no way today to drive a request as "bob who lacks admin:read" through Playwright without building a new fixture (write an acl.yaml into a temp project, spawn a server, and set a principal header per request), which is a substantially larger job than the 's' effort estimate assumes.

    The consequence is that the plan's stated proof for AC2/AC7 at the integration layer is not actually available at the cost assumed. Worse, the plan simultaneously proposes a DashboardView.vue unit test as the other frontend proof — but no DashboardView.test.ts exists (frontend/src/views/ has only AnalyzeView, KanbanView x2, SettingsView tests), so that file would be new too, and a mounted-component test with a stubbed API client cannot prove the endpoint wiring either.

    So as written, every claimed proof that the SPA reads /_dashboard rather than schemaStore.dashboard is either nonexistent or newly-built at unestimated cost.
severity: significant
resolution: 'Test Plan restructured into three layers, each claiming only what it can prove. Layer 1 (Go handler tests) proves the filtering policy AC1-AC7 and carries the RR-XYO03L/RR-CWWJGW canaries. Layer 2 is a NEW frontend/src/views/DashboardView.test.ts (acknowledged as new — no such file exists) mocking the api/dashboard.ts client to prove the SPA reads the endpoint and handles the empty/loading states. Layer 3 demotes E2E to a regression guard: the existing dashboard.spec.ts must stay green after the endpoint swap, and no gated-card E2E is attempted. A multi-principal E2E fixture is now explicitly listed under OUT of scope. Effort raised s → m to reflect the previously unestimated frontend test work.'
status: addressed
---

## Recommended resolution

Split the claim into what each layer can actually prove, and stop asserting E2E
is the proof of SPA wiring:

1. **Go handler tests** (`dashboard_permission_test.go`) prove the filtering
policy — AC1-AC7. These are cheap, mirror `nav_permission_test.go`, and are
where the RR-XYO03L/RR-CWWJGW canaries belong. Unchanged from the plan.
2. **SPA wiring** is proved by a `DashboardView.test.ts` that mocks the
`api/dashboard.ts` client and asserts the view renders the *returned* cards and
the empty state — plus that it does **not** read `schemaStore.dashboard` for the
card list. Acknowledge this is a NEW test file.
3. **E2E**: keep the existing `dashboard.spec.ts` green (cards still render for
the default ungated project) as a regression guard on the endpoint swap. Do
**not** promise a gated-card E2E in this ticket — building a multi-principal
Playwright fixture is its own piece of work and should be a follow-up ticket if
wanted.

Update the plan's Test Plan and the effort note accordingly.
