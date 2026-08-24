---
id: RR-DRIFT1
type: review-response
title: "Vitest drift-guard asserted its own copy, not the registry"
severity: critical
status: addressed
finding: "The Vitest half of the RR-Z0GGTO drift guard (registry.widgetTable.test.ts) built a FRESH registry via defineWidgetRegistry() and re-registered ten widgets from a hand-written list inside the test. It never imported defaultRegistry and never read buildDefaultRegistry, so it asserted its own copy of the registrations while its comment claimed the opposite. VERIFIED by mutation: changing textarea's supportedPropertyTypes in registry.ts to ['string','integer'] left all 12 tests GREEN; adding an entirely new widget to buildDefaultRegistry also left all 12 GREEN. The Go half was genuinely load-bearing, so the guard was one-sided in the direction that matters least: the registry is the side that actually decides what renders. Worse than no guard, because the comment told the next reader the risk was handled."
resolution: "Exported WIDGET_REGISTRATIONS from registry.ts as the single array buildDefaultRegistry consumes, and rewrote the test to assert THAT against the fixture. Re-ran both mutations: each now FAILS. Also replaced the near-vacuous second test block (it asserted fixture-key formatting, not resolvability) with one that resolves every fixture name against defaultRegistry and proves it is not the unknown-name fallback."
---
