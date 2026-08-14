---
id: RR-TIO1XP
type: review-response
title: SPA change adds a second sequential round-trip before any card query can start
finding: |-
    Today DashboardView reads cards synchronously from schemaStore.dashboard (already loaded at app mount from /_config) and fires all card searches in parallel from onMounted — loadData() at DashboardView.vue:21-43 maps over cards.value immediately.

    Moving the card list to /_dashboard makes the card queries depend on a NEW network round-trip that cannot start until the dashboard config resolves: fetch /_dashboard → then fan out N searchEntities calls. That serialises what is currently parallel-after-mount and adds a full RTT to time-to-first-card on every dashboard load, for every user, to deliver a filter that most deployments (no acl.yaml) will never use.

    The plan's SPA section does not mention this at all — it only says the view "fetches /_dashboard instead of reading schemaStore.dashboard" and notes that indices stay dense. It also does not say whether the fetch is cached across navigations (schemaStore.dashboard is loaded once at mount; a per-visit fetch is a behaviour change on repeat visits to /dashboard).
severity: minor
resolution: 'Both sub-decisions are now explicit in the plan''s SPA section rather than left to accident. Caching: /_dashboard is loaded once into the schema store alongside the existing mount-time /_config load, not per visit — preserving today''s free repeat visits to /dashboard and matching the established store pattern. Loading state: `loading` must cover the dashboard-config fetch, not just the card queries, because the new all-filtered empty state and the not-yet-loaded state are visually identical — without this every dashboard load flashes ''no cards'' first. That flicker is now pinned by the new DashboardView.test.ts and listed in the risk table. The added round-trip is accepted and documented as a tradeoff (one RTT against N parallel searches, confined to first load by the store cache).'
status: addressed
---

## Recommended resolution

Not a blocker — correctness first, and the added RTT is small against N search
queries. But decide it explicitly rather than by accident:

1. **State the tradeoff in the plan** so it is a decision, not a regression
someone discovers later in a perf ticket.
2. **Choose a caching posture.** Either load `/_dashboard` into a store once
(mirroring how `/_config` is loaded at mount, keeping repeat visits free), or
fetch per visit and accept it. Prefer the store, since it matches the existing
`schemaStore` pattern and preserves current repeat-visit behaviour.
3. **Keep the loading state honest** — `loading` should cover the config fetch
too, otherwise the view flashes an "empty dashboard" state between mount and the
config arriving, which looks exactly like the all-cards-filtered state added by
AC7.

Point 3 is the one that actually matters: the new empty state and the
not-yet-loaded state are visually identical, so getting the ordering wrong makes
every dashboard load flicker "no cards" first.
