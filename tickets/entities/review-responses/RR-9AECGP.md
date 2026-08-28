---
id: RR-9AECGP
type: review-response
title: A /_dashboard failure bricks the entire SPA boot
finding: |-
    frontend/src/stores/schema.ts put getDashboard() inside the boot Promise.all alongside getSchema()/getConfig(). doLoad() re-throws on catch, and App.vue turns a rejected schemaStore.load() into the full-screen 'Failed to load application' state instead of rendering <RouterView/>. So ANY failure of this one endpoint takes down the sidebar, every list, every form and every view — not just the dashboard.

    Proven, not asserted: with getSchema/getConfig resolving and getDashboard rejecting with a 404, store.load() rejects, store.loaded stays false, and store.navigation is [] — the successfully-fetched config is discarded too.

    The realistic trigger is a newer SPA bundle served against an older rela-server, where /api/v1/_dashboard does not exist and 404s. The operator sees a dead app with no indication which endpoint died. Before this change, `dashboard` was one optional key on /_config; this commit promoted it to a hard boot dependency — for a UX filter that most deployments (no acl.yaml) never exercise.
severity: critical
resolution: |-
    The fetch now settles to a benign value instead of failing the boot: `getDashboard().catch(() => undefined)`. A rejection leaves store.dashboard undefined, so DashboardView renders its empty state while the sidebar, lists and forms load normally — the degradation lands where it belongs. A comment at the call site states plainly that this is NOT a boot dependency and names the newer-SPA-vs-older-server case, so nobody 'tidies' the catch away.

    Pinned by a new schema.test.ts case, 'degrades to an empty dashboard when /_dashboard fails, without failing the boot': it asserts load() RESOLVES, loaded is true, error is null, navigation is fully populated, and only dashboard is undefined. Mutation-verified — restoring the throwing form fails it.
status: addressed
---
