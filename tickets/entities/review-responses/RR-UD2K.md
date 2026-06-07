---
id: RR-UD2K
type: review-response
title: EntityDetail cohesion is roughly neutral
finding: |
  ~30 LoC of cards/list template removed; ~35 LoC of helpers (getPropertyDef, synthDef, widgetForField) added. Complexity didn't decrease -- it moved from explicit template branches (easy to grep, easy to reason about) to script-block helpers operating on synthetic types (harder to grep). The win is "form and view go through the same widget for the same type." The loss is "EntityDetail now owns three private heuristics about how view-side fields map to property defs."
severity: minor
status: addressed
resolution: |
  Addressed as a byproduct of RR-UD2B. synthDef and widgetForField are deleted from EntityDetail. The view-side routing heuristic lives in frontend/src/widgets/viewRouting.ts (viewFieldRoutingHint), a single-purpose module owned by the widgets directory rather than EntityDetail. PropertyDisplay's defForProp lie is also gone -- it now uses resolveFromHint when there's no schema def. EntityDetail keeps only fieldRowsFor (the precompute) which is intrinsic to view-side rendering and small enough not to deserve extraction. Net: shared logic lives in /widgets/, consumer components only bind the precomputed rows to templates.
---
