---
id: RR-CLZB5I
type: review-response
title: 404-when-no-dashboard contradicts AC7's empty-state and has no precedent in the sidebar analogue
finding: |-
    The plan specifies the new endpoint returns "404 when no dashboard: is configured" and separately that an all-cards-filtered dashboard returns "200 with an empty card list". The Edge Cases section then claims 404 is "unchanged from today's behaviour" — but that is not today's behaviour of any endpoint, because /_dashboard does not exist today. Today the SPA reads schemaStore.dashboard from /_config, which simply omits the key (`json:"dashboard,omitempty"`, responses.go:261) and the view falls back to `dashboardConfig.value?.cards || []` (DashboardView.vue:18) and renders an empty grid. So the plan invents a 404 and then describes it as the status quo.

    This also diverges from the analogue it claims to mirror: handleV1Sidebar (views_handler.go:192-257) never 404s — it returns 200 with `navigation: []` when nothing survives, and initialises `navigation := make([]v1.SidebarGroup, 0)` at line 209 specifically so the JSON is `[]` and not `null`.

    The practical cost is that the SPA now needs a 404 branch in DashboardView for a case that is not an error, and 404 is indistinguishable from a genuinely-missing route — which will be the actual failure mode if the endpoint registration is ever wrong.
severity: minor
resolution: 'Dropped the 404 entirely. The endpoint now always returns 200 with cards initialised via make([]v1.DashboardCard, 0), so a missing dashboard:, an authored cards: [], and an all-filtered result are one behaviour producing JSON [] rather than null — mirroring handleV1Sidebar''s make([]v1.SidebarGroup, 0) at views_handler.go:209. AC7 was rewritten to demand 200 + [] (never null, never 404) and the three edge cases were collapsed into one. This removes the SPA branch for a non-error state and keeps 404 meaning ''broken route''. The plan''s incorrect claim that 404 was ''unchanged from today''s behaviour'' is also gone — today /_config simply omits the key via omitempty and the view falls back to an empty array.'
status: addressed
---

## Recommended resolution

Return **200 with an empty card list** in both cases (no `dashboard:`
configured, and every card filtered), mirroring `handleV1Sidebar`:

- Initialise `cards := make([]v1.DashboardCard, 0)` so the wire carries `[]`,
never `null` — the sidebar handler does exactly this at `views_handler.go:209`
and the plan's AC7 already demands "not 404, not null".
- Title/description come from `s.Cfg.Dashboard` when present, empty otherwise.
- The SPA then has ONE code path: render the returned cards, show the empty
state when there are none. No 404 branch, and a real 404 keeps meaning "route is
broken", which is what it should mean.

This also collapses two of the plan's Edge Cases into one behaviour, which is
the simpler design.
