---
id: RR-UD1G
type: review-response
title: MultiSelectWidget is dead in display mode under the chosen decision
finding: |
  The "scalar widgets, display mode loops" decision (correctly chosen) means a multi-select field is rendered by looping and invoking SelectWidget per value. So MultiSelectWidget never gets called in display mode. The ticket lists "MultiSelectWidget: row of Badge per item" in scope but that branch is unreachable. Either delete the requirement or change the resolution: in display mode, defaultWidgetFor collapses multi-select to select. The asymmetry matters for TKT-IHCY7 -- flipping to inline-edit needs to re-resolve the widget, not just flip the mode prop.
severity: significant
resolution: |
  Plan revised. Reversed the "scalar widgets, display mode loops" decision. Widgets now own their own multiplicity: MultiSelectWidget receives the full array on both edit (TagSelect with array) and display (row of Badges, looping internally). Display-mode callers do NOT loop over field.values -- they call the resolved widget once with the full field value. This pushes the multi-vs-single complexity into one widget instead of every caller, and lets TKT-IHCY7 flip mode without re-resolving.
status: addressed
---
