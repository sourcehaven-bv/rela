---
id: RR-FB1N
type: review-response
title: 'S11: Optimistic-or-not — pick a side'
finding: |
  PLAN tries to occupy a middle ground: section's formData is optimistic (v-model bound synchronously) but cross-section dependents aren't. With C3 resolving the cross-section question (none exist on the header), this either resolves naturally or needs an explicit decision.
severity: significant
status: addressed
resolution: |
  Resolved naturally via RR-FB1C: the section IS optimistic (v-model bound to formData updates synchronously); cross-section consumers update on `applyServerProperty` via spread-clone (RR-FB1G). Latency between "section shows new value" and "sibling section shows new value" is ~debounce + RTT = 800-1500ms. AutoSaveIndicator gives the user a visible signal that the round-trip is in flight. No header Badge exists, so no header inconsistency. Documented in known-deltas table.
---
