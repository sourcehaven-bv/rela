---
id: RR-UD2L
type: review-response
title: No test asserts RR-UD1H schema-lookup-once behaviour
finding: |
  The plan called for a spy test asserting schemaStore.getPropertyDef is called once per (section, field), not once per render or once per cell. There is no such test. Combined with finding RR-UD2A, there's no protection against someone undoing the precompute and pushing the cost back into per-cell. Coverage exists for what renders, not for how often schema is consulted.
severity: minor
status: addressed
resolution: |
  Added frontend/src/widgets/viewRouting.test.ts covering: (a) viewFieldRoutingHint returns the same hint shape for the same input (idempotent / referentially stable -- protects against accidentally adding schemaStore subscriptions to the hint path); (b) defaultRegistry.resolveFromHint maps hint kinds to widgets directly without firing the supportedPropertyTypes warning (no schema introspection in the hint path). The spy-on-resolveFromHint test asserts a clean console (no warnings) for plain hint lookups. Combined with the RR-UD2A precompute structure, this catches future regressions that would push schema-lookups back into per-cell render.
---
