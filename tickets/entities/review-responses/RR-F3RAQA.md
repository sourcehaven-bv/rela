---
id: RR-F3RAQA
type: review-response
title: The singleton composable cannot use useLiveSubscription as specified
finding: useLiveSubscription registers onBeforeUnmount and creates a new per-instance key on every call (useLiveSubscription.ts:152-178). Called from a module-level singleton outside a component, it warns and never unregisters. After a 404 and then a 200 it would register a new key. Map insertion order then puts that key last, and send() keeps only the first 16 entries (useLiveSubscription.ts:59), so the badge subscription is the first one dropped on a busy page. Use registerLive with a fixed key such as 'inbox-summary' directly, or model the summary as a Pinia Colada query (the useNavStatus.ts pattern), which already shares one fetch between the sidebar and the page.
severity: minor
resolution: The badge is now a Pinia Colada query with one registerLive under the fixed key inbox-summary owned by Sidebar.vue; useLiveSubscription is used only inside the page component.
status: addressed
---
