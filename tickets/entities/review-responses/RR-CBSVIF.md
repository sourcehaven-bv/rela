---
id: RR-CBSVIF
type: review-response
title: 'Plan misstates permission gating: permission: lives on NavigationEntry and is a UX filter, not enforcement'
finding: 'The plan''s Negative Tests say ''Missing permission: → 403 naming the permission'' and Security says ''permission: gating on the gantt, consistent with other view types''. Both are wrong about how this codebase works. Calendar and Kanban config types have NO Permission field — permission: lives on NavigationEntry (dataentryconfig/config.go:887), DashboardCard (:992), commands (:1128) and documents (:1196). It is enforced by permitsGatedUIElement (internal/dataentry/views_handler.go:318) which only filters SIDEBAR ENTRIES; views_handler.go:260-317 documents that this is a UX filter that ''enforces nothing'' — /api/v1/_config still serves the whole block to every principal (config is not secret) and the URL stays reachable, returning ACL-scoped rows. So there is no 403 to test for. The real enforcement is row-level: the gantt endpoint must return only ACL-visible entities. The plan should replace the 403 test with a sidebar-visibility test (mirroring TestSidebar_CalendarHiddenWithoutPermission, internal/dataentry/calendar_sidebar_test.go:90) plus row-level ACL tests.'
severity: significant
resolution: 'Plan updated: Security Considerations now states permission: is on NavigationEntry (config.go:845, :887), NOT on the view type, and is a UX filter only (views_handler.go:260-341). The fictional ''missing permission → 403'' negative test was removed, with an explicit inline note recording why so it is not reintroduced. Test plan gains a gantt_sidebar_test.go row mirroring calendar_sidebar_test.go:61,90 (href /gantt/<key>, icon, hidden-without-permission). Files-to-modify now includes NavigationEntry.Gantt near config.go:845 and the views_handler.go:359 sidebar arm, which the original plan had omitted entirely.'
status: addressed
---

## Finding

Two related errors in the plan.

**1. `permission:` is not a field on the view type.**

`Calendar` and `Kanban` have no `Permission` field. It lives on:

- `NavigationEntry.Permission` — `dataentryconfig/config.go:887`
- `DashboardCard.Permission` — `:992`
- command `:1128`, document `:1196`

So `gantts.<id>.permission` would simply not exist. The gate is declared on the
**navigation entry pointing at** the gantt.

**2. It is a UX filter, not enforcement — so the planned 403 test is
untestable.**

`permitsGatedUIElement` (`internal/dataentry/views_handler.go:318`) is called
only from `handleV1Sidebar` (`:214`, `:230` via `permitsNavEntry` `:256`) and
from `dashboard_handler.go:44`. The doc comment at `views_handler.go:260-317`
states plainly that it hides menu entries and **enforces nothing**:
`/api/v1/_config` serves the entire block to every principal, and the URL stays
reachable and returns ACL-scoped rows.

This is consistent with CLAUDE.md — "the configuration is not a secret; the data
is", and the settled decision that the sidebar menu is principal-independent
apart from gating.

## Consequence for the plan

The Negative Test "Missing `permission:` → 403 naming the permission" describes
behaviour that does not and should not exist. Delete it.

Note the fail-closed detail worth preserving: `permitsGatedUIElement` returns
**false for a nil ACL**, and true for `NopACL`/`ReadOnlyACL`; only
`*acl.Declarative` consults `readGateFromContext(ctx).HoldsPermission(...)`
(`views_handler.go:341`).

## Resolution

Replace with two tests:

1. **Sidebar visibility** — mirror
`TestSidebar_CalendarHiddenWithoutPermission`
(`internal/dataentry/calendar_sidebar_test.go:90`): build an `acl.Declarative`
via `mustNewACL`, request with `gateCtxFor(aliceCtx(), …)`, assert the gantt
entry is absent. Plus the positive `TestSidebar_GanttEntry` asserting href
`/gantt/<key>` and icon.
2. **Row-level ACL** — the real enforcement: the endpoint returns only visible
entities, and roll-up folds only those (already covered by AC7 and RR-Y7MINP).
